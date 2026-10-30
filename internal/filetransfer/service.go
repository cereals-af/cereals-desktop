package filetransfer

import (
	"context"
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/cereals-af/cereals-desktop/internal/session"
	"github.com/google/uuid"
)

type selectedFile struct {
	Selection
	sessionID, path string
	info            os.FileInfo
	expires         time.Time
}
type job struct {
	mu        sync.Mutex
	progress  Progress
	ctx       context.Context
	cancel    context.CancelFunc
	touched   time.Time
	sessionID string
}
type Service struct {
	ctx        context.Context
	cancel     context.CancelFunc
	sessions   Sessions
	resolver   Resolver
	dialogs    Dialogs
	mu         sync.Mutex
	selections map[string]selectedFile
	jobs       map[string]*job
	workers    sync.WaitGroup
	background bool
	active     int
}

func NewService(ctx context.Context, sessions Sessions, resolver Resolver, dialogs Dialogs) *Service {
	ctx, cancel := context.WithCancel(ctx)
	s := &Service{ctx: ctx, cancel: cancel, sessions: sessions, resolver: resolver, dialogs: dialogs, selections: make(map[string]selectedFile), jobs: make(map[string]*job)}
	s.workers.Go(s.reap)
	return s
}

func (s *Service) SelectUpload(ctx context.Context, sessionID string) (Selection, error) {
	target, err := s.sessions.FileTarget(sessionID)
	if err != nil {
		return Selection{}, err
	}
	filename, err := s.dialogs.PickUpload()
	if err != nil {
		return Selection{}, errors.New("无法打开本地文件选择器")
	}
	if filename == "" {
		return Selection{}, nil
	}
	if ctx.Err() != nil || target.Context.Err() != nil {
		return Selection{}, errors.New("会话已关闭，文件选择已取消")
	}
	info, err := os.Stat(filename)
	if err != nil {
		return Selection{}, errors.New("无法读取所选本地文件")
	}
	if !info.Mode().IsRegular() || info.Size() > MaxFileSize {
		return Selection{}, errors.New("请选择不超过 2 GiB 的普通文件")
	}
	name := filepath.Base(filename)
	if strings.ContainsAny(name, "/\\") || strings.ContainsFunc(name, unicode.IsControl) {
		return Selection{}, errors.New("文件名包含不支持的字符")
	}
	selection := Selection{ID: uuid.NewString(), Name: name, Bytes: info.Size()}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.Err() != nil || ctx.Err() != nil || target.Context.Err() != nil {
		return Selection{}, errors.New("会话已关闭，文件选择已取消")
	}
	s.pruneSelections(time.Now())
	if len(s.selections) >= 16 {
		return Selection{}, errors.New("待上传文件过多，请关闭不用的传输面板")
	}
	s.selections[selection.ID] = selectedFile{Selection: selection, sessionID: sessionID, path: filename, info: info, expires: time.Now().Add(10 * time.Minute)}
	return selection, nil
}
func (s *Service) ReleaseSelection(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.selections, id)
}

func validPath(value string) bool {
	return strings.HasPrefix(value, "/") && len(value) <= 4096 && !strings.ContainsFunc(value, unicode.IsControl)
}

func (s *Service) Start(ctx context.Context, request Request) (string, error) {
	if (request.Direction != "upload" && request.Direction != "download") || !validPath(request.RemotePath) {
		return "", errors.New("请选择上传或下载，并填写容器内的绝对路径")
	}
	request.RemotePath = path.Clean(request.RemotePath)
	target, err := s.sessions.FileTarget(request.SessionID)
	if err != nil {
		return "", err
	}
	client, err := s.resolver.FileClient(target.Request.Target.ClusterID)
	if err != nil {
		return "", err
	}
	if client.ConnectionID() != target.ConnectionID {
		return "", errors.New("集群连接已改变，请重新打开终端")
	}
	source, destination, name, err := s.prepare(request)
	if err != nil {
		return "", err
	}
	if request.Direction == "download" && destination == "" {
		return "", nil
	}
	if ctx.Err() != nil || target.Context.Err() != nil {
		return "", errors.New("文件传输已取消")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.Err() != nil || len(s.jobs) >= 16 {
		return "", errors.New("文件传输服务已关闭或保留任务过多")
	}
	for _, existing := range s.jobs {
		existing.mu.Lock()
		done := existing.progress.Done
		existing.mu.Unlock()
		if !done {
			if existing.sessionID == request.SessionID {
				return "", errors.New("当前会话已有文件正在传输")
			}
		}
	}
	if s.active >= 4 {
		return "", errors.New("最多同时传输 4 个文件")
	}
	lifetime, cancel := context.WithTimeout(s.ctx, 30*time.Minute)
	id := uuid.NewString()
	j := &job{ctx: lifetime, cancel: cancel, sessionID: request.SessionID, touched: time.Now(), progress: Progress{ID: id, State: "connecting", Name: name, RemotePath: request.RemotePath}}
	if request.Direction == "upload" {
		j.progress.RemotePath = path.Join(request.RemotePath, source.Name)
		j.progress.Total = source.Bytes
	}
	s.jobs[id] = j
	s.active++
	s.workers.Go(func() { s.run(j, client, target, request, source, destination) })
	return id, nil
}

func (s *Service) prepare(request Request) (selectedFile, string, string, error) {
	if request.Direction == "upload" {
		s.mu.Lock()
		source, ok := s.selections[request.SelectionID]
		s.mu.Unlock()
		if !ok || source.sessionID != request.SessionID || time.Now().After(source.expires) {
			return selectedFile{}, "", "", errors.New("文件选择已过期，请重新选择本地文件")
		}
		return source, "", source.Name, nil
	}
	if request.RemotePath == "/" || strings.HasSuffix(request.RemotePath, "/") {
		return selectedFile{}, "", "", errors.New("请指定普通文件的完整路径")
	}
	name := path.Base(request.RemotePath)
	destination, err := s.dialogs.SaveFile(name)
	if err != nil {
		return selectedFile{}, "", "", errors.New("无法打开本地保存对话框")
	}
	return selectedFile{}, destination, name, nil
}

func (s *Service) run(j *job, client Client, target session.FileTarget, request Request, source selectedFile, destination string) {
	defer j.cancel()
	defer func() { s.mu.Lock(); s.active--; s.mu.Unlock() }()
	stop := context.AfterFunc(target.Context, j.cancel)
	defer stop()
	if target.Context.Err() != nil {
		j.cancel()
	}
	monitorDone := make(chan struct{})
	monitorExited := make(chan struct{})
	defer func() { close(monitorDone); j.cancel(); <-monitorExited }()
	go func() { defer close(monitorExited); monitor(j, client, target, monitorDone) }()
	err := checkPod(j.ctx, client, target)
	if err == nil {
		j.mu.Lock()
		j.progress.State = "running"
		j.mu.Unlock()
		if request.Direction == "upload" {
			err = upload(j, client, target.Request, source, request.RemotePath, request.Overwrite)
		} else {
			err = download(j, client, target.Request, request.RemotePath, destination)
		}
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	j.progress.Done = true
	switch {
	case err == nil:
		j.progress.State = "completed"
		j.progress.Message = "文件传输完成"
	case errors.Is(err, ErrExists):
		j.progress.State = "conflict"
		j.progress.Message = ErrExists.Error()
	case j.ctx.Err() != nil:
		j.progress.State = "cancelled"
		j.progress.Message = "传输已取消、超时或目标连接已结束；未完成的本地文件已清理"
	case err != nil:
		j.progress.State = "error"
		j.progress.Message = err.Error()
	}
}

func checkPod(ctx context.Context, client Client, target session.FileTarget) error {
	check, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	pod, err := client.Pod(check, target.Request.Target)
	if err != nil {
		return err
	}
	if pod.UID != target.Request.UID {
		return errors.New("Pod 已被重建，文件传输已停止")
	}
	for _, container := range pod.Containers {
		if container.Name == target.Request.Container && container.State == "running" {
			return nil
		}
	}
	return errors.New("目标容器未运行，无法传输文件")
}
func monitor(j *job, client Client, target session.FileTarget, done <-chan struct{}) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-j.ctx.Done():
			return
		case <-done:
			return
		case <-client.SessionDone():
			j.cancel()
			return
		case <-ticker.C:
			if checkPod(j.ctx, client, target) != nil {
				j.cancel()
				return
			}
		}
	}
}

func (s *Service) Poll(id string) (Progress, error) {
	s.mu.Lock()
	j := s.jobs[id]
	if j != nil {
		j.touched = time.Now()
	}
	s.mu.Unlock()
	if j == nil {
		return Progress{}, errors.New("文件传输任务不存在或已过期")
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.progress, nil
}
func (s *Service) Cancel(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j := s.jobs[id]; j != nil {
		j.cancel()
	}
}
func (s *Service) Close(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j := s.jobs[id]; j != nil {
		j.cancel()
		delete(s.jobs, id)
	}
}
func (s *Service) pruneSelections(now time.Time) {
	for id, file := range s.selections {
		if now.After(file.expires) {
			delete(s.selections, id)
		}
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
			s.mu.Lock()
			s.pruneSelections(now)
			if !s.background {
				for id, j := range s.jobs {
					if now.Sub(j.touched) > 45*time.Second {
						j.cancel()
						delete(s.jobs, id)
					}
				}
			}
			s.mu.Unlock()
		}
	}
}

//wails:ignore
func (s *Service) SetBackground(hidden bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.background = hidden
	if !hidden {
		for _, j := range s.jobs {
			j.touched = time.Now()
		}
	}
}
func (s *Service) ServiceShutdown() error {
	s.mu.Lock()
	s.cancel()
	s.mu.Unlock()
	s.workers.Wait()
	s.mu.Lock()
	defer s.mu.Unlock()
	clear(s.jobs)
	clear(s.selections)
	return nil
}
