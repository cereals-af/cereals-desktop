package observe

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/resource"
	"github.com/google/uuid"
)

type streamKey struct {
	Query        resource.Query
	ConnectionID string
}
type stream struct {
	key    streamKey
	cancel context.CancelFunc
	refs   int
	update Update
	notify chan struct{}
}
type subscription struct {
	stream  *stream
	touched time.Time
}

type Service struct {
	ctx        context.Context
	cancel     context.CancelFunc
	resolver   Resolver
	mu         sync.Mutex
	streams    map[streamKey]*stream
	subs       map[string]subscription
	workers    sync.WaitGroup
	background bool
}

func NewService(ctx context.Context, resolver Resolver) *Service {
	ctx, cancel := context.WithCancel(ctx)
	s := &Service{ctx: ctx, cancel: cancel, resolver: resolver, streams: make(map[streamKey]*stream), subs: make(map[string]subscription)}
	s.workers.Go(s.reap)
	return s
}

// Subscribe owns a lease independent of the short Wails call. Poll renews it;
// crashed or unmounted views release the stream within one minute.
func (s *Service) Subscribe(query resource.Query, resourceVersion string) (string, error) {
	if query.Version == "" || query.Resource == "" || resourceVersion == "" {
		return "", errors.New("请先读取列表，再开始实时更新")
	}
	query.Continue = ""
	client, err := s.resolver.WatchClient(query.ClusterID)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.Err() != nil {
		return "", errors.New("实时服务已关闭")
	}
	if len(s.subs) >= 128 {
		return "", errors.New("实时订阅数量达到上限")
	}
	key := streamKey{Query: query, ConnectionID: client.ConnectionID()}
	shared := s.streams[key]
	if shared == nil {
		if len(s.streams) >= 32 {
			return "", errors.New("实时连接数量达到上限")
		}
		ctx, cancel := context.WithCancel(s.ctx)
		shared = &stream{key: key, cancel: cancel, update: Update{Revision: 1, State: "connecting"}, notify: make(chan struct{})}
		s.streams[key] = shared
		s.workers.Go(func() {
			err := client.Observe(ctx, query, resourceVersion, func(change Change) { s.publish(shared, change) })
			if ctx.Err() == nil {
				message := "实时连接已结束，请重新开启实时更新"
				if err != nil {
					message = err.Error()
				}
				s.publish(shared, Change{State: "error", Message: message})
			}
		})
	}
	shared.refs++
	id := uuid.NewString()
	s.subs[id] = subscription{stream: shared, touched: time.Now()}
	return id, nil
}

func (s *Service) publish(shared *stream, change Change) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.streams[shared.key] != shared {
		return
	}
	shared.update.Revision++
	if change.Dirty {
		shared.update.Changes++
	}
	if change.State != "" {
		shared.update.State, shared.update.Message = change.State, change.Message
	}
	close(shared.notify)
	shared.notify = make(chan struct{})
}

// Poll returns one coalesced notification, never an unbounded queue of objects.
func (s *Service) Poll(ctx context.Context, id string, revision uint64) (Update, error) {
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	for {
		s.mu.Lock()
		sub, ok := s.subs[id]
		if !ok {
			s.mu.Unlock()
			return Update{}, errors.New("实时订阅已过期")
		}
		sub.touched = time.Now()
		s.subs[id] = sub
		update, changed := sub.stream.update, sub.stream.notify
		s.mu.Unlock()
		if update.Revision != revision {
			return update, nil
		}
		select {
		case <-changed:
		case <-timer.C:
			return update, nil
		case <-ctx.Done():
			return Update{}, ctx.Err()
		case <-s.ctx.Done():
			return Update{}, errors.New("实时服务已关闭")
		}
	}
}

func (s *Service) Unsubscribe(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removeLocked(id)
}

func (s *Service) removeLocked(id string) {
	sub, exists := s.subs[id]
	if !exists {
		return
	}
	delete(s.subs, id)
	sub.stream.refs--
	if sub.stream.refs == 0 {
		sub.stream.cancel()
		delete(s.streams, sub.stream.key)
		close(sub.stream.notify)
	}
}

func (s *Service) reap() {
	ticker := time.NewTicker(15 * time.Second)
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

// SetBackground retains subscriptions while a hidden native window cannot poll.
//
//wails:ignore
func (s *Service) SetBackground(hidden bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for id, sub := range s.subs {
		if !s.background && hidden && now.Sub(sub.touched) > 45*time.Second {
			s.removeLocked(id)
		} else if s.background && !hidden {
			sub.touched = now
			s.subs[id] = sub
		}
	}
	s.background = hidden
}

func (s *Service) expire(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.background {
		return
	}
	for id, sub := range s.subs {
		if now.Sub(sub.touched) > 45*time.Second {
			s.removeLocked(id)
		}
	}
}

func (s *Service) ServiceShutdown() error {
	s.cancel()
	s.mu.Lock()
	for id := range s.subs {
		s.removeLocked(id)
	}
	s.mu.Unlock()
	s.workers.Wait()
	return nil
}
