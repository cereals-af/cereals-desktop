package filetransfer

import (
	"archive/tar"
	"context"
	"errors"
	"io"
	"os"
	"path"
	"strconv"

	"github.com/cereals-af/cereals-desktop/internal/localfile"
	"github.com/cereals-af/cereals-desktop/internal/session"
	"github.com/google/uuid"
)

// Scripts are constant; all user paths are positional arguments, never shell code.
// Upload stages beside the target. ln provides no-clobber publication; mv replaces
// only after explicit confirmation. A cancelled stream never truncates the target.
const uploadScript = `set -eu
for tool in tar rm mv ln; do command -v "$tool" >/dev/null 2>&1 || exit 19; done
dir=$1; staged=$2; name=$3; replace=$4
test -d "$dir" || exit 20
target="$dir/$name"; partial="$dir/$staged"
if test -e "$target" || test -L "$target"; then
  test "$replace" = true || exit 17
  test -f "$target" && ! test -L "$target" || exit 18
fi
trap 'rm -f -- "$partial"' EXIT HUP INT TERM
tar -xmf - -C "$dir"
test -f "$partial" && ! test -L "$partial" || exit 18
if test "$replace" = true; then
  if test -e "$target" || test -L "$target"; then test -f "$target" && ! test -L "$target" || exit 18; fi
  mv -f -- "$partial" "$target"
else
  ln -- "$partial" "$target" || exit 17
fi
`
const downloadScript = `set -eu
command -v tar >/dev/null 2>&1 || exit 19
test -f "$1/$2" && ! test -L "$1/$2" || exit 18
tar -cf - -C "$1" "./$2"
`

type progressWriter struct {
	writer io.Writer
	job    *job
}

func contextPipe(j *job, reader *io.PipeReader, writer *io.PipeWriter) func() bool {
	return context.AfterFunc(j.ctx, func() { reader.CloseWithError(j.ctx.Err()); writer.CloseWithError(j.ctx.Err()) })
}
func (w progressWriter) Write(p []byte) (int, error) {
	if err := w.job.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := w.writer.Write(p)
	w.job.mu.Lock()
	w.job.progress.Bytes += int64(n)
	w.job.mu.Unlock()
	return n, err
}

func upload(j *job, client Client, target session.Request, source selectedFile, directory string, overwrite bool) error {
	file, err := os.Open(source.path)
	if err != nil {
		return errors.New("无法打开所选本地文件")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || !os.SameFile(info, source.info) || info.Size() != source.Bytes || !info.ModTime().Equal(source.info.ModTime()) {
		return errors.New("本地文件已变化，请重新选择后上传")
	}
	staged := ".cereals-upload-" + uuid.NewString() + ".part"
	reader, writer := io.Pipe()
	defer reader.Close()
	stop := contextPipe(j, reader, writer)
	defer stop()
	produced := make(chan error, 1)
	go func() {
		archive := tar.NewWriter(writer)
		err := archive.WriteHeader(&tar.Header{Name: staged, Mode: int64(info.Mode().Perm() & 0777), Size: source.Bytes, Typeflag: tar.TypeReg})
		if err == nil {
			_, err = io.CopyN(progressWriter{archive, j}, file, source.Bytes)
		}
		if err == nil {
			err = archive.Close()
		}
		writer.CloseWithError(err)
		produced <- err
	}()
	err = client.FileExec(j.ctx, target, []string{"/bin/sh", "-c", uploadScript, "cereals-upload", directory, staged, source.Name, strconv.FormatBool(overwrite)}, reader, io.Discard)
	reader.CloseWithError(err)
	producerError := <-produced
	if err != nil {
		return err
	}
	return producerError
}

func download(j *job, client Client, target session.Request, remote, destination string) error {
	return localfile.Write(j.ctx, destination, func(output io.Writer) error {
		execCtx, cancelExec := context.WithCancel(j.ctx)
		defer cancelExec()
		reader, writer := io.Pipe()
		defer reader.Close()
		stop := contextPipe(j, reader, writer)
		defer stop()
		done := make(chan error, 1)
		go func() {
			err := client.FileExec(execCtx, target, []string{"/bin/sh", "-c", downloadScript, "cereals-download", path.Dir(remote), path.Base(remote)}, nil, writer)
			writer.CloseWithError(err)
			done <- err
		}()
		err := extractFile(reader, progressWriter{output, j}, path.Base(remote), func(size int64) { j.mu.Lock(); j.progress.Total = size; j.mu.Unlock() })
		reader.CloseWithError(err)
		if err != nil {
			cancelExec()
		}
		remoteError := <-done
		if remoteError != nil && !errors.Is(remoteError, context.Canceled) {
			return remoteError
		}
		return err
	})
}

// Only one regular member with the selected basename is accepted. No archive
// paths, symbolic links or hard links are ever used to create local files.
func extractFile(input io.Reader, output io.Writer, name string, total func(int64)) error {
	archive := tar.NewReader(input)
	header, err := archive.Next()
	if err != nil {
		return errors.New("无法读取容器文件归档，请检查文件权限和 tar 支持")
	}
	if (header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA) || (header.Name != name && header.Name != "./"+name) || header.Size < 0 || header.Size > MaxFileSize {
		return errors.New("仅支持不超过 2 GiB 的普通文件，归档内容与所选文件不匹配")
	}
	total(header.Size)
	if _, err = io.CopyN(output, archive, header.Size); err != nil {
		return err
	}
	if _, err = archive.Next(); err != io.EOF {
		return errors.New("容器返回了额外或不完整的归档内容，下载已取消")
	}
	// Consume the stream's EOF so the remote exit status is checked before commit.
	n, err := io.Copy(io.Discard, io.LimitReader(input, (1<<20)+1))
	if n > 1<<20 {
		return errors.New("容器归档尾部超出允许大小，下载已取消")
	}
	return err
}
