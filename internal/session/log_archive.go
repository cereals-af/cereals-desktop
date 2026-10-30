package session

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

const logArchiveLimit = 100 << 20

var errArchiveFull = errors.New("日志归档已达到 100 MiB，采集已停止；请下载后重新开启会话，已接收内容完整保留")

// logArchive is separate from the renderer's acknowledged ring buffer. Snapshots
// hold a fixed byte boundary and keep the file alive through a concurrent close.
type logArchive struct {
	mu      sync.Mutex
	file    *os.File
	size    int64
	limit   int64
	refs    int
	closed  bool
	failure error
}

func newLogArchive() (*logArchive, error) {
	file, err := newArchiveFile()
	if err != nil {
		return nil, errors.New("无法创建日志临时归档，请检查本地临时目录")
	}
	return &logArchive{file: file, limit: logArchiveLimit}, nil
}

func (a *logArchive) Write(p []byte) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return 0, errors.New("日志会话已关闭")
	}
	if a.failure != nil {
		return 0, a.failure
	}
	if int64(len(p)) > a.limit-a.size {
		a.failure = errArchiveFull
		return 0, a.failure
	}
	n, err := a.file.WriteAt(p, a.size)
	a.size += int64(n)
	if err != nil {
		a.failure = errors.New("日志临时归档写入失败，采集已停止；已归档内容仍可下载")
	}
	return n, err
}

func (a *logArchive) err() error { a.mu.Lock(); defer a.mu.Unlock(); return a.failure }
func (a *logArchive) close()     { a.mu.Lock(); defer a.mu.Unlock(); a.closed = true; a.cleanup() }
func (a *logArchive) cleanup() {
	if !a.closed || a.refs != 0 || a.file == nil {
		return
	}
	a.file.Close()
	a.file = nil
}

type LogSnapshot struct {
	Reader   io.Reader
	Bytes    int64
	Filename string
	Context  context.Context
	release  func()
}

func (s *LogSnapshot) Close() {
	if s.release != nil {
		s.release()
	}
}

// OpenLog captures the export boundary before opening a native save dialog.
//
//wails:ignore
func (s *Service) OpenLog(id string) (*LogSnapshot, error) {
	op, err := s.get(id)
	if err != nil {
		return nil, err
	}
	if op.archive == nil {
		return nil, errors.New("该会话没有日志归档")
	}
	a := op.archive
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed || a.size == 0 {
		return nil, errors.New("当前会话暂无可下载的日志")
	}
	a.refs++
	var once sync.Once
	return &LogSnapshot{Reader: io.NewSectionReader(a.file, 0, a.size), Bytes: a.size, Context: op.owner,
		Filename: fmt.Sprintf("%s_%s_%s_%s.log", op.request.Target.Namespace, op.request.Target.Name, op.request.Container, time.Now().Format("20060102-150405")),
		release:  func() { once.Do(func() { a.mu.Lock(); defer a.mu.Unlock(); a.refs--; a.cleanup() }) }}, nil
}

// FileTarget identifies the immutable owner of an independent file transfer.
type FileTarget struct {
	Request      Request
	ConnectionID string
	Context      context.Context
}

//wails:ignore
func (s *Service) FileTarget(id string) (FileTarget, error) {
	op, err := s.get(id)
	if err != nil {
		return FileTarget{}, err
	}
	if op.request.Kind != "exec" || op.owner.Err() != nil {
		return FileTarget{}, errors.New("请从有效的终端会话传输文件")
	}
	return FileTarget{Request: op.request, ConnectionID: op.connectionID, Context: op.owner}, nil
}

// Touch retains completed logs while their view is still mounted.
func (s *Service) Touch(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	op := s.sessions[id]
	if op == nil {
		return errors.New("会话不存在或已过期")
	}
	op.touched = time.Now()
	return nil
}
