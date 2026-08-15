// return不了float和double啊。

package main

import "C"
import (
	"fmt"
)

//export Add
func Add(a, b C.int) C.int {
	result := int(a) + int(b)
	fmt.Printf("DLL: Add function called with %d and %d, returning %d\n", int(a), int(b), result)
	return C.int(result)
}

//export AddFloat
func AddFloat(a, b C.float) C.float {
	result := float32(a) + float32(b)
	fmt.Printf("DLL: AddFloat function called with %f and %f, returning %f\n", float32(a), float32(b), result)
	return C.float(result)
}

//export AddDouble
func AddDouble(a, b C.double) C.double {
	result := float64(a) + float64(b)
	fmt.Printf("DLL: AddDouble function called with %f and %f, returning %f\n", float64(a), float64(b), result)
	return C.double(result)
}

//export ConcatString
func ConcatString(a, b *C.char) *C.char {
	result := C.GoString(a) + C.GoString(b)
	fmt.Printf("DLL: ConcatString function called with '%s' and '%s', returning '%s'\n", C.GoString(a), C.GoString(b), result)
	return C.CString(result)
}

//export SampleFunction
func SampleFunction(intVal C.int, floatVal C.float, doubleVal C.double, strVal *C.char, boolVal C.int, resultPtr *C.double) C.int {
	goInt := int(intVal)
	goFloat := float32(floatVal)
	goDouble := float64(doubleVal)
	goString := C.GoString(strVal)
	goBool := boolVal != 0

	fmt.Printf("DLL: SampleFunction called with int=%d, float=%f, double=%f, string=%s, bool=%v\n",
		goInt, goFloat, goDouble, goString, goBool)

	result := float64(goInt) + float64(goFloat) + goDouble

	if goBool {
		result *= 2
	}

	fmt.Printf("DLL: SampleFunction calculated result: %f\n", result)

	// 这个可以
	*resultPtr = C.double(result)

	return C.int(result)
}

func main() {}
