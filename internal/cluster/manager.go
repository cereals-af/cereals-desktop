package cluster

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/cereals-af/cereals-desktop/internal/buildinfo"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	"k8s.io/client-go/tools/clientcmd"
)

type session struct {
	client  resource.Client
	cancel  context.CancelFunc
	version string
}

type Manager struct {
	mu          sync.RWMutex
	lifetime    context.Context
	factory     Factory
	sources     Sources
	preferences Preferences
	preference  Preference
	entries     map[string]entry
	sessions    map[string]session
	pending     map[string]context.CancelFunc
	warnings    []string
	closed      bool
}

func NewManager(ctx context.Context, factory Factory, sources Sources, preferences Preferences) *Manager {
	m := &Manager{lifetime: ctx, factory: factory, sources: sources, preferences: preferences,
		entries: make(map[string]entry), sessions: make(map[string]session),
		pending: make(map[string]context.CancelFunc), warnings: []string{}}
	paths, err := sources.List()
	if err != nil {
		m.warnings = append(m.warnings, "无法读取已保存的配置路径，请检查应用设置中的 sources.json")
		return m
	}
	for _, path := range paths {
		entries, err := loadEntries(path)
		if err != nil {
			m.warnings = append(m.warnings, fmt.Sprintf("配置 %s 加载失败，请重新导入", filepath.Base(path)))
			continue
		}
		for _, item := range entries {
			m.entries[item.profile.ID] = item
		}
	}
	m.loadPreference()
	return m
}

func (m *Manager) State() State {
	m.mu.RLock()
	defer m.mu.RUnlock()
	state := State{Profiles: make([]Profile, 0, len(m.entries)), Warnings: append([]string{}, m.warnings...)}
	for id, item := range m.entries {
		profile := item.profile
		if connected, ok := m.sessions[id]; ok {
			profile.Connected, profile.Version = true, connected.version
		}
		state.Profiles = append(state.Profiles, profile)
	}
	if profile, ok := m.defaultProfileLocked(); ok {
		state.DefaultClusterID = profile.ID
		state.DefaultNeedsConfirmation = (profile.ExecCommand != "" || profile.InsecureTLS) && m.preference.ApprovedID != profile.ID
	} else if m.preference.Source != "" {
		state.Warnings = append(state.Warnings, "默认集群配置已不可用，请重新导入或选择默认集群")
	}
	sort.Slice(state.Profiles, func(i, j int) bool { return state.Profiles[i].Context < state.Profiles[j].Context })
	return state
}

func (m *Manager) Import(path string) (State, error) {
	entries, err := loadEntries(path)
	if err != nil {
		return State{}, err
	}
	if err := m.sources.Add(entries[0].profile.Source); err != nil {
		return State{}, fmt.Errorf("保存配置路径: %w", err)
	}
	m.mu.Lock()
	for id, existing := range m.entries {
		if existing.profile.Source == entries[0].profile.Source {
			m.disconnectLocked(id)
			delete(m.entries, id)
		}
	}
	for _, item := range entries {
		m.entries[item.profile.ID] = item
	}
	m.ensureSingleDefaultLocked()
	m.mu.Unlock()
	return m.State(), nil
}

func (m *Manager) Connect(ctx context.Context, request ConnectRequest) (Connection, error) {
	item, sessionCtx, cancel, err := m.beginConnection(request)
	if err != nil {
		return Connection{}, err
	}
	ctx, cancelRequest := context.WithTimeout(ctx, 15*time.Second)
	defer cancelRequest()
	stopRequest := context.AfterFunc(ctx, cancel)
	defer stopRequest()
	config, err := clientcmd.NewNonInteractiveClientConfig(*item.config.DeepCopy(), item.profile.Context, &clientcmd.ConfigOverrides{}, nil).ClientConfig()
	if err != nil {
		cancel()
		return Connection{}, errors.New("无法加载集群凭据，请检查证书文件、认证配置和插件路径")
	}
	config.Timeout, config.QPS, config.Burst = 15*time.Second, 10, 20
	config.UserAgent = "cereals/" + buildinfo.Version
	started := time.Now()
	client, err := m.factory.New(sessionCtx, config)
	if err != nil {
		cancel()
		return Connection{}, err
	}
	version, err := client.Version(ctx)
	if err != nil {
		cancel()
		client.Close()
		return Connection{}, err
	}
	// Stop the request hook before publishing a successful, long-lived connection.
	if !stopRequest() || ctx.Err() != nil {
		cancel()
		client.Close()
		return Connection{}, errors.New("连接已取消，请重新连接")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || sessionCtx.Err() != nil {
		cancel()
		client.Close()
		return Connection{}, errors.New("连接已取消，请重新连接")
	}
	m.sessions[request.ClusterID] = session{client: client, cancel: cancel, version: version}
	m.rememberDefaultApprovalLocked(item.profile)
	delete(m.pending, request.ClusterID)
	return Connection{ClusterID: request.ClusterID, Version: version, LatencyMS: time.Since(started).Milliseconds()}, nil
}

func (m *Manager) beginConnection(request ConnectRequest) (entry, context.Context, context.CancelFunc, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	item, ok := m.entries[request.ClusterID]
	if !ok || m.closed {
		return entry{}, nil, nil, errors.New("集群配置不存在，请重新导入")
	}
	if item.profile.ExecCommand != "" && !request.TrustExec {
		return entry{}, nil, nil, errors.New("此配置需要运行本地认证插件，请先确认信任")
	}
	m.disconnectLocked(request.ClusterID)
	ctx, cancel := context.WithCancel(m.lifetime)
	m.pending[request.ClusterID] = cancel
	return item, ctx, cancel, nil
}

func (m *Manager) Client(id string) (resource.Client, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	connected, ok := m.sessions[id]
	if !ok {
		return nil, errors.New("集群尚未连接，请先连接对应 Context")
	}
	return connected.client, nil
}

func (m *Manager) Disconnect(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.disconnectLocked(id)
}

func (m *Manager) disconnectLocked(id string) {
	if cancel, ok := m.pending[id]; ok {
		cancel()
		delete(m.pending, id)
	}
	if connected, ok := m.sessions[id]; ok {
		connected.cancel()
		connected.client.Close()
		delete(m.sessions, id)
	}
}

func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	for id := range m.entries {
		m.disconnectLocked(id)
	}
}
