package cluster

import "errors"

// Preference identifies a context independently of credential rotations. Approval
// is deliberately bound to the complete kubeconfig fingerprint (Profile.ID).
type Preference struct {
	Source     string `json:"source"`
	Context    string `json:"context"`
	ApprovedID string `json:"approvedID,omitempty"`
}

type Preferences interface {
	Load() (Preference, error)
	Save(Preference) error
}

func (m *Manager) loadPreference() {
	preference, err := m.preferences.Load()
	if err != nil {
		m.warnings = append(m.warnings, "无法读取默认集群设置，请重新设置默认集群")
		return
	}
	m.preference = preference
	m.ensureSingleDefaultLocked()
}

func (m *Manager) defaultProfileLocked() (Profile, bool) {
	for _, entry := range m.entries {
		if entry.profile.Source == m.preference.Source && entry.profile.Context == m.preference.Context {
			return entry.profile, true
		}
	}
	if len(m.entries) == 1 {
		for _, entry := range m.entries {
			return entry.profile, true
		}
	}
	return Profile{}, false
}

// Caller holds mu, except during construction before the manager is published.
func (m *Manager) ensureSingleDefaultLocked() {
	if len(m.entries) != 1 {
		return
	}
	profile, ok := m.defaultProfileLocked()
	if !ok {
		return
	}
	if m.preference.Source == profile.Source && m.preference.Context == profile.Context {
		return
	}
	next := Preference{Source: profile.Source, Context: profile.Context}
	if err := m.preferences.Save(next); err != nil {
		m.warnings = append(m.warnings, "无法保存默认集群设置，本次仍以唯一集群作为默认")
		return
	}
	m.preference = next
}

func (m *Manager) SetDefault(clusterID string) (State, error) {
	m.mu.Lock()
	entry, ok := m.entries[clusterID]
	if !ok || m.closed {
		m.mu.Unlock()
		return State{}, errors.New("默认集群不存在，请重新选择")
	}
	next := Preference{Source: entry.profile.Source, Context: entry.profile.Context}
	if _, connected := m.sessions[clusterID]; connected || m.preference.ApprovedID == clusterID {
		next.ApprovedID = clusterID
	}
	if err := m.preferences.Save(next); err != nil {
		m.mu.Unlock()
		return State{}, errors.New("无法保存默认集群设置，请检查本地设置目录")
	}
	m.preference = next
	m.mu.Unlock()
	return m.State(), nil
}

func (m *Manager) rememberDefaultApprovalLocked(profile Profile) {
	current, ok := m.defaultProfileLocked()
	if !ok || current.ID != profile.ID || m.preference.ApprovedID == profile.ID {
		return
	}
	next := Preference{Source: profile.Source, Context: profile.Context, ApprovedID: profile.ID}
	if err := m.preferences.Save(next); err != nil {
		m.warnings = append(m.warnings, "连接已建立，但无法保存默认集群的自动连接确认")
		return
	}
	m.preference = next
}
