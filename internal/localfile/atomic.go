// Package localfile writes user-selected exports without exposing partial files.
package localfile

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
)

func Write(ctx context.Context, destination string, fill func(io.Writer) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(destination), ".cereals-export-*")
	if err != nil {
		return errors.New("无法在所选目录创建文件，请检查目录和写入权限")
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if err = fill(&contextWriter{ctx, file}); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return errors.New("无法将导出文件写入磁盘")
	}
	if err = file.Close(); err != nil {
		return errors.New("无法完成导出文件")
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = os.Rename(file.Name(), destination); err != nil {
		return errors.New("无法保存文件，请检查目标是否被占用或只读")
	}
	return nil
}

type contextWriter struct {
	ctx    context.Context
	writer io.Writer
}

func (w *contextWriter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	return w.writer.Write(p)
}
