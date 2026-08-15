package deadline

import (
	"context"
	"fmt"
	"time"
)

// TimeoutRunner 定义了一个超时运行器
type TimeoutRunner struct {
	Timeout time.Duration
}

// Run 接受一个函数fn，并在超时时间内执行它
func (tr *TimeoutRunner) Run(fn func() error) error {
	ctx, cancel := context.WithTimeout(context.Background(), tr.Timeout)
	defer cancel()

	doneChan := make(chan error, 1)

	go func() {
		// 执行传入的函数
		doneChan <- fn()
	}()

	select {
	case err := <-doneChan:
		// 函数执行完成
		return err
	case <-ctx.Done():
		// 超时或者被取消
		return fmt.Errorf("operation timed out")
	}
}

func RunWithTimeout(deadline time.Duration, fn func() error) error {
	return (&TimeoutRunner{Timeout: deadline}).Run(fn)
}
