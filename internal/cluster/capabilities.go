package cluster

import (
	"errors"

	"github.com/cereals-af/cereals-desktop/internal/filetransfer"
	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"github.com/cereals-af/cereals-desktop/internal/metrics"
	"github.com/cereals-af/cereals-desktop/internal/node"
	"github.com/cereals-af/cereals-desktop/internal/observe"
	"github.com/cereals-af/cereals-desktop/internal/overview"
	"github.com/cereals-af/cereals-desktop/internal/relations"
	"github.com/cereals-af/cereals-desktop/internal/schema"
	podsession "github.com/cereals-af/cereals-desktop/internal/session"
	"github.com/cereals-af/cereals-desktop/internal/storage"
)

func (m *Manager) MetricsReader(id string) (metrics.Reader, error) {
	client, err := m.Client(id)
	if err != nil {
		return nil, err
	}
	reader, ok := client.(metrics.Reader)
	if !ok {
		return nil, errors.New("此连接不支持资源用量查询")
	}
	return reader, nil
}

func (m *Manager) OverviewClient(id string) (overview.Client, error) {
	client, err := m.Client(id)
	if err != nil {
		return nil, err
	}
	reader, ok := client.(overview.Client)
	if !ok {
		return nil, errors.New("此连接不支持集群统计")
	}
	return reader, nil
}

func (m *Manager) StorageReader(id string) (storage.Reader, error) {
	client, err := m.Client(id)
	if err != nil {
		return nil, err
	}
	reader, ok := client.(storage.Reader)
	if !ok {
		return nil, errors.New("此连接不支持存储查询")
	}
	return reader, nil
}

func (m *Manager) SchemaClient(id string) (schema.Client, error) {
	client, err := m.Client(id)
	if err != nil {
		return nil, err
	}
	reader, ok := client.(schema.Client)
	if !ok {
		return nil, errors.New("此连接不支持 Schema 查询")
	}
	return reader, nil
}

func (m *Manager) RelationClient(id string) (relations.Client, error) {
	client, err := m.Client(id)
	if err != nil {
		return nil, err
	}
	reader, ok := client.(relations.Client)
	if !ok {
		return nil, errors.New("此连接不支持关联查询")
	}
	return reader, nil
}

func (m *Manager) ManifestClient(id string) (manifest.Client, error) {
	client, err := m.Client(id)
	if err != nil {
		return nil, err
	}
	writer, ok := client.(manifest.Client)
	if !ok {
		return nil, errors.New("此连接不支持资源写入")
	}
	return writer, nil
}

func (m *Manager) WatchClient(id string) (observe.Client, error) {
	client, err := m.Client(id)
	if err != nil {
		return nil, err
	}
	watcher, ok := client.(observe.Client)
	if !ok {
		return nil, errors.New("此连接不支持实时更新")
	}
	return watcher, nil
}

func (m *Manager) SessionClient(id string) (podsession.Client, error) {
	client, err := m.Client(id)
	if err != nil {
		return nil, err
	}
	runner, ok := client.(podsession.Client)
	if !ok {
		return nil, errors.New("此连接不支持 Pod 会话")
	}
	return runner, nil
}

func (m *Manager) NodeClient(id string) (node.Client, error) {
	client, err := m.Client(id)
	if err != nil {
		return nil, err
	}
	reader, ok := client.(node.Client)
	if !ok {
		return nil, errors.New("此连接不支持节点维护")
	}
	return reader, nil
}

func (m *Manager) FileClient(id string) (filetransfer.Client, error) {
	client, err := m.Client(id)
	if err != nil {
		return nil, err
	}
	runner, ok := client.(filetransfer.Client)
	if !ok {
		return nil, errors.New("此连接不支持文件传输")
	}
	return runner, nil
}
