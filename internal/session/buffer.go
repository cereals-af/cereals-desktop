package session

import (
	"context"
	"encoding/base64"
	"errors"
	"sync"
)

const outputLimit = 256 << 10
const chunkLimit = 32 << 10
const inputLimit = 64 << 10

// outputBuffer blocks the producer at a fixed byte limit. Poll is replayable:
// data is discarded only when a subsequent cursor acknowledges receipt.
type outputBuffer struct {
	ctx           context.Context
	mu            sync.Mutex
	bytes         []byte
	base, offered uint64
	changed       chan struct{}
	update        Update
}

func newOutput(ctx context.Context) *outputBuffer {
	return &outputBuffer{ctx: ctx, changed: make(chan struct{}), update: Update{State: "connecting", ExitCode: -1}}
}
func (b *outputBuffer) signal() { close(b.changed); b.changed = make(chan struct{}) }
func (b *outputBuffer) Write(p []byte) (int, error) {
	written := 0
	for len(p) > 0 {
		b.mu.Lock()
		if err := b.ctx.Err(); err != nil {
			b.mu.Unlock()
			return written, err
		}
		n := min(len(p), outputLimit-len(b.bytes))
		if n > 0 {
			b.bytes = append(b.bytes, p[:n]...)
			p = p[n:]
			written += n
			b.signal()
		}
		changed := b.changed
		b.mu.Unlock()
		if n == 0 {
			select {
			case <-changed:
			case <-b.ctx.Done():
				return written, b.ctx.Err()
			}
		}
	}
	return written, nil
}
func (b *outputBuffer) status(state, message string, port uint16, code int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.update.State, b.update.Message, b.update.LocalPort, b.update.ExitCode = state, message, port, code
	b.signal()
}
func (b *outputBuffer) read(cursor uint64) (Update, <-chan struct{}, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if cursor < b.base || cursor > b.offered {
		return Update{}, nil, errors.New("会话读取位置无效，请重新打开会话")
	}
	if cursor > b.base {
		n := int(cursor - b.base)
		copy(b.bytes, b.bytes[n:])
		b.bytes = b.bytes[:len(b.bytes)-n]
		b.base = cursor
		b.signal()
	}
	result := b.update
	n := min(chunkLimit, len(b.bytes))
	result.Data = base64.StdEncoding.EncodeToString(b.bytes[:n])
	result.Cursor = b.base + uint64(n)
	b.offered = max(b.offered, result.Cursor)
	result.Done = result.State != "connecting" && result.State != "running" && n == len(b.bytes)
	return result, b.changed, nil
}

type inputBuffer struct {
	ctx     context.Context
	mu      sync.Mutex
	bytes   []byte
	changed chan struct{}
}

func newInput(ctx context.Context) *inputBuffer {
	return &inputBuffer{ctx: ctx, changed: make(chan struct{})}
}
func (b *inputBuffer) send(data []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.ctx.Err() != nil {
		return errors.New("终端已结束")
	}
	if len(b.bytes)+len(data) > inputLimit {
		return errors.New("终端输入过快，输入队列已满")
	}
	b.bytes = append(b.bytes, data...)
	close(b.changed)
	b.changed = make(chan struct{})
	return nil
}
func (b *inputBuffer) Read(p []byte) (int, error) {
	for {
		b.mu.Lock()
		if b.ctx.Err() != nil {
			b.mu.Unlock()
			return 0, b.ctx.Err()
		}
		if len(b.bytes) > 0 {
			n := copy(p, b.bytes)
			copy(b.bytes, b.bytes[n:])
			b.bytes = b.bytes[:len(b.bytes)-n]
			b.mu.Unlock()
			return n, nil
		}
		changed := b.changed
		b.mu.Unlock()
		select {
		case <-changed:
		case <-b.ctx.Done():
			return 0, b.ctx.Err()
		}
	}
}
