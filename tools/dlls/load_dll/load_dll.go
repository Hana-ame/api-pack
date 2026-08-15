// https://claude.ai/chat/e177632c-dbd5-4053-a639-00a6f7a487f8

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

func main() {
	// 加载 DLL 文件
	dll, err := syscall.LoadDLL("user32.dll")
	if err != nil {
		panic(err)
	}
	defer dll.Release()

	// 查找 MessageBoxW 函数
	proc, err := dll.FindProc("MessageBoxW")
	if err != nil {
		panic(err)
	}

	// 调用 MessageBoxW 函数
	title, _ := syscall.UTF16PtrFromString("标题")
	text, _ := syscall.UTF16PtrFromString("Hello, 这是一个来自 Go 的消息框!")
	ret, _, _ := proc.Call(
		0,
		uintptr(unsafe.Pointer(text)),
		uintptr(unsafe.Pointer(title)),
		0,
	)

	fmt.Printf("MessageBox 返回值: %d\n", ret)
}
