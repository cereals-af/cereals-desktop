// Package desktop exposes narrow Wails bindings over application services.
package desktop

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/cereals-af/cereals-desktop/internal/cluster"
)

type FilePicker interface{ PickKubeconfig() (string, error) }

type WorkspaceService struct {
	manager *cluster.Manager
	picker  FilePicker
}

func NewWorkspaceService(manager *cluster.Manager, picker FilePicker) *WorkspaceService {
	return &WorkspaceService{manager: manager, picker: picker}
}

func (s *WorkspaceService) SetDefault(clusterID string) (cluster.State, error) {
	return s.manager.SetDefault(clusterID)
}

func (s *WorkspaceService) State() cluster.State { return s.manager.State() }

func (s *WorkspaceService) ImportFile() (cluster.State, error) {
	path, err := s.picker.PickKubeconfig()
	if err != nil {
		return cluster.State{}, errors.New("无法打开文件选择器")
	}
	if path == "" {
		return s.manager.State(), nil
	}
	return s.manager.Import(path)
}

func (s *WorkspaceService) ImportDefault() (cluster.State, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return cluster.State{}, errors.New("无法定位用户目录")
	}
	return s.manager.Import(filepath.Join(home, ".kube", "config"))
}

func (s *WorkspaceService) Connect(ctx context.Context, request cluster.ConnectRequest) (cluster.Connection, error) {
	return s.manager.Connect(ctx, request)
}

func (s *WorkspaceService) Disconnect(clusterID string) { s.manager.Disconnect(clusterID) }

func (s *WorkspaceService) ServiceShutdown() error {
	s.manager.Close()
	return nil
}
