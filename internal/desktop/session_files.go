package desktop

import (
	"context"
	"errors"
	"io"
	"path/filepath"

	"github.com/cereals-af/cereals-desktop/internal/localfile"
	"github.com/cereals-af/cereals-desktop/internal/session"
)

type SavePicker interface{ SaveFile(string) (string, error) }
type LogSource interface {
	OpenLog(string) (*session.LogSnapshot, error)
}
type SavedFile struct {
	Name      string `json:"name"`
	Bytes     int64  `json:"bytes"`
	Cancelled bool   `json:"cancelled"`
}
type SessionFilesService struct {
	logs   LogSource
	picker SavePicker
}

func NewSessionFilesService(logs LogSource, picker SavePicker) *SessionFilesService {
	return &SessionFilesService{logs: logs, picker: picker}
}

func (s *SessionFilesService) ExportLogs(ctx context.Context, id string) (SavedFile, error) {
	snapshot, err := s.logs.OpenLog(id)
	if err != nil {
		return SavedFile{}, err
	}
	defer snapshot.Close()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(snapshot.Context, cancel)
	defer stop()
	destination, err := s.picker.SaveFile(snapshot.Filename)
	if err != nil {
		return SavedFile{}, errors.New("无法打开日志保存对话框")
	}
	if destination == "" {
		return SavedFile{Cancelled: true}, nil
	}
	if err = snapshot.Context.Err(); err != nil {
		return SavedFile{}, err
	}
	if err = localfile.Write(ctx, destination, func(w io.Writer) error { _, err := io.Copy(w, snapshot.Reader); return err }); err != nil {
		return SavedFile{}, err
	}
	return SavedFile{Name: filepath.Base(destination), Bytes: snapshot.Bytes}, nil
}
