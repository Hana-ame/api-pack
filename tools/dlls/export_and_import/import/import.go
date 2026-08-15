// https://github.com/golang/go/issues/3588
// 下次试试在param里面用pointer引用。

package main

import "C"
import (
	"fmt"
	"math"
	"syscall"
	"unsafe"
)

const DLL_PATH = "../export/example.dll"

func main() {
	dll, err := syscall.LoadDLL(DLL_PATH)
	if err != nil {
		panic(err)
	}
	defer dll.Release()

	// Test Add function
	addProc, _ := dll.FindProc("Add")
	addRet, _, _ := addProc.Call(uintptr(5), uintptr(3))
	fmt.Printf("Add result: %d\n", int(addRet))

	// Test AddFloat function
	addFloatProc, _ := dll.FindProc("AddFloat")
	a, b := float32(2.5), float32(3.7)
	addFloatRet, _, _ := addFloatProc.Call(uintptr(math.Float32bits(a)), uintptr(math.Float32bits(b)))
	fmt.Printf("SampleFunction raw return value: %v\n", addFloatRet)
	fmt.Printf("AddFloat result: %f\n", math.Float32frombits(uint32(addFloatRet)))

	// Test AddDouble function
	addDoubleProc, _ := dll.FindProc("AddDouble")
	c, d := 2.5, 3.7
	addDoubleRet, _, _ := addDoubleProc.Call(uintptr(math.Float64bits(c)), uintptr(math.Float64bits(d)))
	fmt.Printf("SampleFunction raw return value: %v\n", addFloatRet)
	fmt.Printf("AddDouble result: %f\n", math.Float64frombits(uint64(addDoubleRet)))

	// Test ConcatString function
	concatStringProc, _ := dll.FindProc("ConcatString")
	str1, _ := syscall.BytePtrFromString("Hello, ")
	str2, _ := syscall.BytePtrFromString("World!")
	concatStringRet, _, _ := concatStringProc.Call(uintptr(unsafe.Pointer(str1)), uintptr(unsafe.Pointer(str2)))
	// 将返回的C字符串转换为Go字符串
	fmt.Printf("ConcatString result: %s\n", C.GoString((*C.char)(unsafe.Pointer(concatStringRet)))) // must be that, or use &result instead.

	// Test SampleFunction
	sampleProc, _ := dll.FindProc("SampleFunction")
	intArg := 42
	floatArg := float32(3.14)
	doubleArg := 2.71828
	strArg, _ := syscall.BytePtrFromString("Hello, DLL!")
	boolArg := true
	var result float64

	sampleRet, _, err := sampleProc.Call(
		uintptr(intArg),
		uintptr(math.Float32bits(floatArg)),
		uintptr(math.Float64bits(doubleArg)),
		uintptr(unsafe.Pointer(strArg)),
		uintptr(boolToInt(boolArg)),
		uintptr(unsafe.Pointer(&result)),
	)

	if err != syscall.Errno(0) {
		fmt.Println("SampleFunction syscall returned error:", err)
	}

	fmt.Printf("SampleFunction raw return value: %v\n", sampleRet)
	sampleResult := math.Float64frombits(uint64(sampleRet))
	fmt.Printf("SampleFunction result (as float64): %f\n", sampleResult)
	fmt.Printf("SampleFunction result (as float64): %f\n", result)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
