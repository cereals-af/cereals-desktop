package node

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

const planTTL = 5 * time.Minute
const taskLease = time.Minute
const maxPods = 500

type pendingPlan struct {
	request      DrainRequest
	plan         Plan
	fingerprints map[string]string
	connectionID string
	cordonToken  string
	expires      time.Time
}
type task struct {
	ctx      context.Context
	cancel   context.CancelFunc
	progress Progress
	touched  time.Time
}
type Service struct {
	ctx      context.Context
	cancel   context.CancelFunc
	resolver Resolver
	writer   Writer
	mu       sync.Mutex
	plans    map[string]pendingPlan
	tasks    map[string]*task
	active   int
	workers  sync.WaitGroup
	interval time.Duration
}

func NewService(ctx context.Context, resolver Resolver, writer Writer) *Service {
	ctx, cancel := context.WithCancel(ctx)
	s := &Service{ctx: ctx, cancel: cancel, resolver: resolver, writer: writer, plans: map[string]pendingPlan{}, tasks: map[string]*task{}, interval: 2 * time.Second}
	s.workers.Go(s.reap)
	return s
}

// DiscardDrain releases an uncommitted plan and its scheduling preview.
func (s *Service) DiscardDrain(token string) {
	s.mu.Lock()
	plan, ok := s.plans[token]
	delete(s.plans, token)
	s.mu.Unlock()
	if ok {
		s.writer.Discard(plan.cordonToken)
	}
}

// StartDrain consumes one immutable plan. Tasks are tied to the connection and a
// renewable view lease, rather than the lifetime of the short desktop request.
func (s *Service) StartDrain(ctx context.Context, token, confirmation string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	plan, ok := s.plans[token]
	if !ok || time.Now().After(plan.expires) {
		return "", errors.New("排空预检已失效，请重新检查")
	}
	if confirmation != plan.request.Target.Name {
		return "", errors.New("请输入完整节点名称确认排空")
	}
	if ctx.Err() != nil || s.ctx.Err() != nil {
		return "", errors.New("操作已取消")
	}
	if len(s.tasks) >= 8 || s.active >= 4 {
		return "", errors.New("最多同时运行 4 个、保留 8 个排空任务，请关闭已结束的任务")
	}
	for _, job := range s.tasks {
		if !job.progress.Done && job.progress.Target.ClusterID == plan.request.Target.ClusterID && job.progress.Target.Name == plan.request.Target.Name {
			return "", errors.New("此节点已有排空任务")
		}
	}
	client, err := s.resolver.NodeClient(plan.request.Target.ClusterID)
	if err != nil {
		return "", err
	}
	if client.ConnectionID() != plan.connectionID {
		return "", errors.New("集群连接已更换，请重新预检")
	}
	delete(s.plans, token)
	lifetime, cancel := context.WithTimeout(s.ctx, time.Duration(plan.request.Options.TimeoutSeconds)*time.Second)
	job := &task{ctx: lifetime, cancel: cancel, touched: time.Now(), progress: Progress{Target: plan.request.Target, State: "starting", Message: "正在复核节点和 Pod 清单", Pods: append([]PodProgress{}, plan.plan.Pods...)}}
	id := uuid.NewString()
	s.tasks[id] = job
	s.active++
	s.workers.Go(func() { s.run(job, plan, client) })
	return id, nil
}

func (s *Service) PollDrain(id string) (Progress, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.tasks[id]
	if !ok {
		return Progress{}, errors.New("排空任务不存在或已过期")
	}
	job.touched = time.Now()
	result := job.progress
	result.Pods = append([]PodProgress{}, result.Pods...)
	return result, nil
}

// CancelDrain stops future work. Accepted evictions cannot be rolled back.
func (s *Service) CancelDrain(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if job, ok := s.tasks[id]; ok {
		job.cancel()
	}
}

func (s *Service) CloseDrain(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if job, ok := s.tasks[id]; ok {
		job.cancel()
		delete(s.tasks, id)
	}
}

func (s *Service) reap() {
	timer := time.NewTicker(5 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case now := <-timer.C:
			s.expire(now)
		}
	}
}

func (s *Service) expire(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for token, plan := range s.plans {
		if now.After(plan.expires) {
			s.writer.Discard(plan.cordonToken)
			delete(s.plans, token)
		}
	}
	for id, job := range s.tasks {
		if !job.progress.Done && now.Sub(job.touched) > taskLease {
			job.cancel()
		}
		if job.progress.Done && now.Sub(job.touched) > 10*time.Minute {
			delete(s.tasks, id)
		}
	}
}

func (s *Service) ServiceShutdown() error {
	s.cancel()
	s.mu.Lock()
	for _, job := range s.tasks {
		job.cancel()
	}
	for token, plan := range s.plans {
		s.writer.Discard(plan.cordonToken)
		delete(s.plans, token)
	}
	s.mu.Unlock()
	s.workers.Wait()
	return nil
}
