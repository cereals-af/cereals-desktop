package manifest

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
)

const previewLifetime = 5 * time.Minute

type ticket struct {
	dependencies []Dependency
	operation    Operation
	deletion     *DeleteRequest
	connectionID string
	expires      time.Time
	timer        *time.Timer
}

func (s *Service) remember(preview Preview, pending ticket) (Preview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Preview{}, errors.New("应用正在关闭")
	}
	if len(s.tickets) >= 16 {
		return Preview{}, errors.New("待确认操作过多，请关闭其他编辑窗口或等待预检过期")
	}
	preview.Token = uuid.NewString()
	pending.expires = time.Now().Add(previewLifetime)
	preview.ExpiresAt = pending.expires.Format(time.RFC3339)
	pending.timer = time.AfterFunc(previewLifetime, func() { s.Discard(preview.Token) })
	s.tickets[preview.Token] = pending
	return preview, nil
}

// Commit consumes a one-use preview. An ambiguous network failure must be checked
// against the cluster before the user prepares a fresh operation; never auto retry.
func (s *Service) Commit(ctx context.Context, token string) (Result, error) {
	s.mu.Lock()
	pending, exists := s.tickets[token]
	delete(s.tickets, token)
	s.mu.Unlock()
	if !exists {
		return Result{}, errors.New("预检已失效或已提交，请重新预检")
	}
	pending.timer.Stop()
	if time.Now().After(pending.expires) {
		return Result{}, errors.New("预检已过期，请重新预检")
	}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	target := pending.operation.Target
	client, err := s.resolver.ManifestClient(target.ClusterID)
	if err != nil {
		return Result{}, err
	}
	if client.ConnectionID() != pending.connectionID {
		return Result{}, errors.New("集群连接已更换，请重新读取和预检")
	}
	if err = checkDependencies(ctx, client, target, pending.dependencies); err != nil {
		return Result{}, err
	}
	if pending.deletion != nil {
		err = client.DeleteObject(ctx, *pending.deletion, false)
	} else {
		_, err = client.WriteObject(ctx, pending.operation, false)
	}
	if err != nil {
		return Result{}, err
	}
	return Result{Target: target, Message: "API Server 已接受请求；请通过资源状态确认实际结果，资源可能仍在调谐或删除中。"}, nil
}

func (s *Service) PreviewDelete(ctx context.Context, request DeleteRequest) (Preview, error) {
	if !slices.Contains([]string{"Background", "Foreground", "Orphan"}, request.Propagation) {
		return Preview{}, errors.New("请选择有效的级联删除策略")
	}
	if request.UseGrace && (request.GraceSeconds < 0 || request.GraceSeconds > 86400) {
		return Preview{}, errors.New("宽限期必须在 0 到 86400 秒之间")
	}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	client, err := s.resolver.ManifestClient(request.Target.ClusterID)
	if err != nil {
		return Preview{}, err
	}
	object, err := client.GetObject(ctx, request.Target)
	if err != nil {
		return Preview{}, err
	}
	if err := checkBaseline(object, request.Baseline); err != nil {
		return Preview{}, err
	}
	clean := editableObject(object)
	if isSecret(request.Target) {
		redact(clean)
	}
	before, err := render(clean)
	if err != nil {
		return Preview{}, err
	}
	if err := client.DeleteObject(ctx, request, true); err != nil {
		return Preview{}, err
	}
	preview := Preview{Target: request.Target, Mode: "delete", Before: before}
	return s.remember(preview, ticket{operation: Operation{Target: request.Target, Mode: "delete"}, deletion: &request, connectionID: client.ConnectionID()})
}

func (s *Service) Discard(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if pending, ok := s.tickets[token]; ok {
		pending.timer.Stop()
		delete(s.tickets, token)
	}
}

func (s *Service) ServiceShutdown() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	for key, pending := range s.tickets {
		pending.timer.Stop()
		delete(s.tickets, key)
	}
	return nil
}
