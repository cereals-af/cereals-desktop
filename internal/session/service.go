package session

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/resource"
	"github.com/google/uuid"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

type operation struct {
	owner        context.Context
	closeOwner   context.CancelFunc
	archive      *logArchive
	request      Request
	connectionID string
	ctx          context.Context
	cancel       context.CancelFunc
	output       *outputBuffer
	input        *inputBuffer
	sizes        chan Size
	touched      time.Time
}
type Service struct {
	ctx        context.Context
	cancel     context.CancelFunc
	resolver   Resolver
	mu         sync.Mutex
	sessions   map[string]*operation
	workers    sync.WaitGroup
	active     int
	background bool
}

func NewService(ctx context.Context, resolver Resolver) *Service {
	ctx, cancel := context.WithCancel(ctx)
	s := &Service{ctx: ctx, cancel: cancel, resolver: resolver, sessions: make(map[string]*operation)}
	s.workers.Go(s.reap)
	return s
}
func (s *Service) Pod(ctx context.Context, target resource.Ref) (Pod, error) {
	if err := validateTarget(target); err != nil {
		return Pod{}, err
	}
	client, err := s.resolver.SessionClient(target.ClusterID)
	if err != nil {
		return Pod{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return client.Pod(ctx, target)
}

// Start creates a session independent of the short desktop call. Poll and Touch
// renew its lease; an abandoned view cannot retain resources indefinitely.
func (s *Service) Start(ctx context.Context, request Request) (string, error) {
	if err := validateTarget(request.Target); err != nil {
		return "", err
	}
	client, err := s.resolver.SessionClient(request.Target.ClusterID)
	if err != nil {
		return "", err
	}
	check, cancelCheck := context.WithTimeout(ctx, 15*time.Second)
	defer cancelCheck()
	pod, err := client.Pod(check, request.Target)
	if err != nil {
		return "", err
	}
	if err = validate(request, pod); err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.Err() != nil || ctx.Err() != nil {
		return "", errors.New("会话创建已取消")
	}
	if len(s.sessions) >= 16 || s.active >= 16 {
		return "", errors.New("最多同时保留 16 个会话，请关闭不用的会话")
	}
	owner, closeOwner := context.WithCancel(s.ctx)
	lifetime, cancel := context.WithCancel(owner)
	var archive *logArchive
	if request.Kind == "logs" {
		archive, err = newLogArchive()
		if err != nil {
			cancel()
			closeOwner()
			return "", err
		}
	}
	op := &operation{owner: owner, closeOwner: closeOwner, archive: archive, request: request, connectionID: client.ConnectionID(), ctx: lifetime, cancel: cancel, output: newOutput(lifetime), input: newInput(lifetime), sizes: make(chan Size, 1), touched: time.Now()}
	op.sizes <- Size{request.Columns, request.Rows}
	id := uuid.NewString()
	s.sessions[id] = op
	s.active++
	s.workers.Go(func() { s.run(op, client) })
	return id, nil
}
func (s *Service) run(op *operation, client Client) {
	defer func() { s.mu.Lock(); s.active--; s.mu.Unlock() }()
	// Connection cancellation must also unblock readers/writers under backpressure.
	watcherDone := make(chan struct{})
	defer close(watcherDone)
	go monitorPod(op, client, watcherDone, 5*time.Second)
	defer op.cancel()
	var output io.Writer = op.output
	if op.archive != nil {
		output = io.MultiWriter(op.archive, op.output)
	}
	outcome, err := client.RunSession(op.ctx, op.request, Streams{Input: op.input, Output: output, Sizes: op.sizes, Ready: func(port uint16) { op.output.status("running", "", port, -1) }})
	state, message := "completed", "会话已结束"
	switch {
	case op.archive != nil && op.archive.err() != nil:
		state, message = "stopped", op.archive.err().Error()
	case op.ctx.Err() != nil:
		state, message = "stopped", "会话已停止或集群连接已断开"
	case err != nil:
		state, message = "error", err.Error()
	case op.request.Kind == "exec":
		message = "远端命令已退出"
	}
	op.output.mu.Lock()
	port := op.output.update.LocalPort
	op.output.mu.Unlock()
	op.output.status(state, message, port, outcome.ExitCode)
}

// A session never follows a same-name replacement. Transient read failures do not
// interrupt the stream; a confirmed deletion or different UID cancels it.
func monitorPod(op *operation, client Client, done <-chan struct{}, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-client.SessionDone():
			op.cancel()
			return
		case <-done:
			return
		case <-op.ctx.Done():
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(op.ctx, 10*time.Second)
			pod, err := client.Pod(ctx, op.request.Target)
			cancel()
			if errors.Is(err, ErrPodGone) || apierrors.IsNotFound(err) || (err == nil && pod.UID != op.request.UID) {
				op.cancel()
				return
			}
		}
	}
}

func (s *Service) get(id string) (*operation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	op := s.sessions[id]
	if op == nil {
		return nil, errors.New("会话不存在或已过期")
	}
	return op, nil
}
func (s *Service) Poll(ctx context.Context, id string, cursor uint64) (Update, error) {
	op, err := s.get(id)
	if err != nil {
		return Update{}, err
	}
	s.mu.Lock()
	op.touched = time.Now()
	s.mu.Unlock()
	timer := time.NewTimer(500 * time.Millisecond)
	defer timer.Stop()
	for {
		update, changed, err := op.output.read(cursor)
		if err != nil || update.Data != "" || update.Done {
			return update, err
		}
		select {
		case <-changed:
		case <-timer.C:
			return update, nil
		case <-ctx.Done():
			return Update{}, ctx.Err()
		case <-s.ctx.Done():
			return Update{}, errors.New("会话服务已关闭")
		}
	}
}
func (s *Service) Input(id, data string) error {
	op, err := s.get(id)
	if err != nil {
		return err
	}
	if op.request.Kind != "exec" {
		return errors.New("该会话不接受终端输入")
	}
	if len(data) > 12<<10 {
		return errors.New("单次终端输入过大")
	}
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return errors.New("终端输入编码无效")
	}
	return op.input.send(decoded)
}
func (s *Service) Resize(id string, size Size) error {
	if err := validateSize(size); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	op := s.sessions[id]
	if op == nil || op.ctx.Err() != nil {
		return errors.New("终端会话已结束")
	}
	if op.request.Kind != "exec" {
		return errors.New("该会话不是终端")
	}
	select {
	case <-op.sizes:
	default:
	}
	op.sizes <- size
	return nil
}
func (s *Service) Stop(id string) {
	if op, err := s.get(id); err == nil {
		op.cancel()
	}
}
func (s *Service) Close(id string) { s.mu.Lock(); defer s.mu.Unlock(); s.remove(id) }
func (s *Service) remove(id string) {
	if op := s.sessions[id]; op != nil {
		op.cancel()
		op.closeOwner()
		if op.archive != nil {
			op.archive.close()
		}
		delete(s.sessions, id)
	}
}
func (s *Service) reap() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case now := <-ticker.C:
			s.expire(now)
		}
	}
}

// SetBackground retains bounded sessions while the native window is hidden.
// Webviews may suspend polling in the background. Explicit close, disconnect,
// Pod replacement and shutdown still cancel sessions through their normal paths.
//
//wails:ignore
func (s *Service) SetBackground(hidden bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if !s.background && hidden {
		for id, op := range s.sessions {
			if now.Sub(op.touched) > 45*time.Second {
				s.remove(id)
			}
		}
	}
	if s.background && !hidden {
		for _, op := range s.sessions {
			op.touched = now
		}
	}
	s.background = hidden
}

func (s *Service) expire(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, op := range s.sessions {
		if now.Sub(op.touched) > 45*time.Second && !s.background {
			s.remove(id)
		}
	}
}
func (s *Service) ServiceShutdown() error {
	s.cancel()
	s.mu.Lock()
	for id := range s.sessions {
		s.remove(id)
	}
	s.mu.Unlock()
	s.workers.Wait()
	return nil
}
