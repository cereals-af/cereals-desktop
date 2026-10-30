package node

import (
	"context"
	"errors"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/formedit"
)

func (s *Service) run(job *task, plan pendingPlan, client Client) {
	defer job.cancel()
	defer s.writer.Discard(plan.cordonToken)
	watcherDone := make(chan struct{})
	defer close(watcherDone)
	go func() {
		select {
		case <-client.SessionDone():
			job.cancel()
		case <-job.ctx.Done():
		case <-watcherDone:
		}
	}()
	err := s.execute(job, plan, client)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active--
	job.progress.Done = true
	switch {
	case errors.Is(job.ctx.Err(), context.DeadlineExceeded):
		job.progress.State = "timeout"
		job.progress.Message = "排空超时；查看各 Pod 的等待状态。节点不会自动解封。"
	case job.ctx.Err() != nil:
		job.progress.State = "cancelled"
		job.progress.Message = "已停止后续操作；已接受的驱逐不会回滚，请刷新确认节点与 Pod 状态。"
	case err != nil:
		job.progress.State = "failed"
		job.progress.Message = err.Error()
	default:
		job.progress.State = "completed"
		job.progress.Message = "已确认的可驱逐 Pod 均已离开；保留项见列表。节点保持封锁，请在维护完成后手动解封。"
	}
}

func (s *Service) execute(job *task, plan pendingPlan, client Client) error {
	if err := job.ctx.Err(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(job.ctx, 30*time.Second)
	pods, err := collectPods(ctx, client, plan.request.Target)
	if err == nil {
		err = checkInventory(plan, pods)
	}
	cancel()
	if err != nil {
		return err
	}
	if err := job.ctx.Err(); err != nil {
		return err
	}
	if _, err = s.writer.Commit(job.ctx, plan.cordonToken); err != nil {
		return err
	}
	s.mu.Lock()
	job.progress.State = "running"
	job.progress.Message = "节点已封锁，正在驱逐并等待 Pod 离开"
	s.mu.Unlock()
	rows := append([]PodProgress{}, plan.plan.Pods...)
	for {
		unfinished := false
		failed := false
		for i := range rows {
			if job.ctx.Err() != nil {
				return job.ctx.Err()
			}
			if rows[i].State == "failed" {
				failed = true
				continue
			}
			if rows[i].State == "done" || rows[i].State == "skipped" {
				continue
			}
			if err := guardNode(job.ctx, client, plan); err != nil {
				return err
			}
			s.advance(job.ctx, client, plan, &rows[i])
			s.mu.Lock()
			job.progress.Pods[i] = rows[i]
			s.mu.Unlock()
			if rows[i].State == "failed" {
				failed = true
			} else if rows[i].State != "done" {
				unfinished = true
			}
		}
		if !unfinished {
			if failed {
				return errors.New("部分 Pod 处理失败；已完成项不会回滚，请检查结果后重新预检")
			}
			return verifyDrained(job.ctx, client, plan)
		}
		timer := time.NewTimer(s.interval)
		select {
		case <-job.ctx.Done():
			timer.Stop()
			return job.ctx.Err()
		case <-timer.C:
		}
	}
}

func guardNode(ctx context.Context, client Reader, plan pendingPlan) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	object, err := readNode(ctx, client, plan.request.Target)
	if err != nil {
		return err
	}
	if string(object.GetUID()) != plan.request.Baseline.UID {
		return errors.New("节点已被重建，停止排空")
	}
	if snapshot := summarizeNode(object); !snapshot.Unschedulable {
		return errors.New("节点已被其他操作解封，停止排空，请重新预检")
	}
	return nil
}

func (s *Service) advance(ctx context.Context, client Client, plan pendingPlan, row *PodProgress) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	target := podTarget(plan.request.Target, row.Pod)
	object, err := client.ReadDrainPod(ctx, target)
	if err != nil {
		row.State = "failed"
		row.Message = err.Error()
		return
	}
	if object == nil || string(object.GetUID()) != row.Pod.UID {
		row.State = "done"
		row.Message = "原 Pod UID 已不存在；同名新对象不会被驱逐"
		return
	}
	digest, err := fingerprint(object)
	if err != nil {
		row.State = "failed"
		row.Message = err.Error()
		return
	}
	if stringAt(object.Object, "spec", "nodeName") != plan.request.Target.Name || digest != plan.fingerprints[row.Pod.UID] {
		row.State = "failed"
		row.Message = "Pod 配置或归属变化，需要重新预检"
		return
	}
	if object.GetDeletionTimestamp() != nil {
		row.State = "waiting"
		row.Message = "等待 Pod 删除完成（可能受宽限期或 finalizer 影响）"
		return
	}
	// An accepted request is never replayed after an ambiguous response. Only an
	// explicit 429 is retried; resourceVersion and UID protect every eviction.
	if row.State == "accepted" {
		row.Message = "驱逐已接受，等待原 Pod 消失"
		return
	}
	result, err := client.EvictPod(ctx, Eviction{Target: target, Baseline: formedit.Baseline(object)})
	if err != nil {
		row.State = "failed"
		row.Message = err.Error() + "；请刷新核实结果后重新预检"
		return
	}
	if result.Waiting {
		row.State = "pending"
		row.Message = "等待 PDB 放行或 API 限流恢复"
		return
	}
	row.State = "accepted"
	row.Message = "驱逐请求已接受，等待 Pod 离开"
}

func verifyDrained(ctx context.Context, client Client, plan pendingPlan) error {
	if err := guardNode(ctx, client, plan); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	pods, err := collectPods(ctx, client, plan.request.Target)
	if err != nil {
		return err
	}
	for _, object := range pods {
		row := classify(summarizePod(object), plan.request.Options)
		if row.State != "skipped" {
			return errors.New("节点仍有 Pod 或出现新 Pod，需要重新预检；不会自动扩大排空范围")
		}
	}
	return nil
}
