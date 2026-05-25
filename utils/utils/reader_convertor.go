// 神人 s3
// 你妈的 Seek failed silently 是吧。
// request.body不支持，所以加一个转换
// ！！！！不用了，指定length就可以了
package tools

import (
	"bytes"
	"errors"
	"fmt"
	"io"
)

type readerSource byte

const (
	READER_VOID = iota
	READER_SOURCE
	READER_BUFFER
)

type CanResetOnceReader struct {
	src readerSource
	io.Reader
	*bytes.Buffer
}

func (r *CanResetOnceReader) Read(p []byte) (n int, err error) {
	switch r.src {
	case READER_SOURCE:
		return r.Reader.Read(p)
	case READER_BUFFER:
		return r.Buffer.Read(p)
	}
	return 0, io.EOF
}

// Seek implements the [io.Seeker] interface.
func (r *CanResetOnceReader) Seek(offset int64, whence int) (int64, error) {
	if r.src == READER_BUFFER {
		return 0, errors.New("tools.CanResetOnceReader: can only reset once")
	}
	var abs int64
	switch whence {
	case io.SeekStart:
		abs = offset
	case io.SeekCurrent:
		abs = offset
	default:
		return 0, fmt.Errorf("tools.CanResetOnceReader: invalid whence %d", whence)
	}
	if abs != 0 {
		return 0, fmt.Errorf("tools.CanResetOnceReader: invalid position %d", abs)
	}
	r.src = READER_BUFFER // 切换到Buffer模式
	return abs, nil
}

func NewCanResetOnceReader(src io.Reader) *CanResetOnceReader {
	buffer := bytes.NewBuffer(nil)
	tee := io.TeeReader(src, buffer) // 分流到管道写入端
	return &CanResetOnceReader{
		src:    READER_SOURCE,
		Reader: tee,
		Buffer: buffer,
	}
}
