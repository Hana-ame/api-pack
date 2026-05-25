package deadline

import (
	"fmt"
	"testing"
	"time"
)

func TestRunnerTimeout(t *testing.T) {
	runner := TimeoutRunner{Timeout: 2 * time.Second}

	// 示例函数，模拟耗时操作
	longRunningTask := func() error {
		time.Sleep(3 * time.Second) // 模拟一个需要3秒完成的任务
		fmt.Println("?")
		return nil
	}

	err := runner.Run(longRunningTask)
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Println("Task completed successfully")
	}

	time.Sleep(10 * time.Second)
}

func TestRunnerDone(t *testing.T) {
	runner := TimeoutRunner{Timeout: 4 * time.Second}

	// 示例函数，模拟耗时操作
	longRunningTask := func() error {
		time.Sleep(3 * time.Second) // 模拟一个需要3秒完成的任务
		fmt.Println("?")
		return nil
	}

	err := runner.Run(longRunningTask)
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Println("Task completed successfully")
	}

	time.Sleep(10 * time.Second)
}

func TestRunner(t *testing.T) {
	// 示例函数，模拟耗时操作
	longRunningTask := func() error {
		time.Sleep(3 * time.Second) // 模拟一个需要3秒完成的任务
		fmt.Println("?")
		return nil
	}
	go func() {
		for {
			time.Sleep(time.Second / 3)
			fmt.Println(time.Now())
		}
	}()
	var err error
	err = RunWithTimeout(time.Second*5, longRunningTask)
	fmt.Println(err)
	err = RunWithTimeout(time.Second*2, longRunningTask)
	fmt.Println(err)
	err = RunWithTimeout(time.Second*3, longRunningTask)
	fmt.Println(err)
}
