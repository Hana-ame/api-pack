package tools

import (
	"fmt"
	"slices"
)

// 扩展的Slice对象
// 拥有带error的Get(index)、GetOrDefault(index)
type Slice[T comparable] []T

func NewSlice[T comparable](e ...T) Slice[T] {
	return Slice[T](e)
}

func (s Slice[T]) ToAny() []any {
	result := make([]any, len(s))
	for i, v := range s {
		result[i] = v
	}
	return result
}

func (s Slice[T]) ToString() []string {
	result := make([]string, len(s))
	for i, v := range s {
		result[i] = fmt.Sprintf("%v", v)
	}
	return result
}
func (a Slice[T]) String() string {
	s := "["
	for _, v := range a {
		s += fmt.Sprintf("%v, ", v)
	}
	s += "]"

	return s
}

// 超过range时，给出dv；无dv，给出该Type默认值
func (s Slice[T]) Get(index int) *result[T] {
	if index < 0 || index >= len(s) {
		var dv T
		return &result[T]{dv, fmt.Errorf("out of range"), false}
	}
	return &result[T]{s[index], nil, false}
}

// 超过range时，给出dv；无dv，给出该Type默认值
func (s Slice[T]) Last() *result[T] {
	if len(s) == 0 {
		var dv T
		return &result[T]{dv, fmt.Errorf("slice is empty"), false}
	}
	return &result[T]{s[len(s)-1], nil, false}
}

func (s Slice[T]) First(dv ...T) *result[T] {
	if len(s) == 0 {
		var dv T
		return &result[T]{dv, fmt.Errorf("slice is empty"), false}
	}
	return &result[T]{s[0], nil, false}
}

func (s Slice[T]) Find(filter func(v T) bool) *result[T] {
	for _, v := range s {
		if filter(v) {
			return &result[T]{v, fmt.Errorf("not found"), false}
		}
	}
	var defaultValue T
	return &result[T]{defaultValue, fmt.Errorf("not found"), false}
}

//	func (s Slice[T]) Map(index i[RT any]nt, defaultValue T) RT {
//		if index < 0 || len(s) >= index {
//			return defaultValue
//		}
//		return s[index]
//	}

func (s Slice[T]) Filter(filter func(v T) bool) Slice[T] {
	result := make([]T, 0, len(s))
	for _, v := range s {
		if filter(v) {
			result = append(result, v)
		}
	}

	return result
}

func (s Slice[T]) Contains(v T) bool {
	return slices.Contains(s, v)
}

func (s Slice[T]) ReverseInPlace() {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i] // 双指针交换元素
	}
}

func MoveToFirstInPlace[T comparable](arr []T, target T) {
	for i, v := range arr {
		if v == target && i != 0 {
			// 将目标元素冒泡到首位
			for j := i; j > 0; j-- {
				arr[j], arr[j-1] = arr[j-1], arr[j]
			}
			return
		}
	}
}

// s.Filter(tools.UnEqual("should not be this","and that")).First()
func UnEqual[T comparable](values ...T) func(v T) bool {
	s := Slice[T](values)
	return func(v T) bool {
		return !s.Contains(v)
	}

	// if len(values) == 0 {
	// 	return func(v T) bool { return true }
	// }
	// if len(values) == 1 {
	// 	return func(v T) bool { return values[0] != v }
	// }
	// return func(v T) bool {
	// 	for _, value := range values {
	// 		if v == value {
	// 			return false
	// 		}
	// 	}
	// 	return true
	// }
}

// 不是指针，不能这么用
// func (s Slice[T]) Push(e T) Slice[T] {
// 	s = append(s, e)
// 	return s
// }
// func (s Slice[T]) Pop() (Slice[T], T) {
// 	e := s.Last()
// 	s = s[:len(s)-1]
// 	return s, e
// }

//	func (s Slice[T]) First(filter func(v T) bool, defaultValue T) (T, error) {
//		for _, v := range s {
//			if filter(v) {
//				return v, nil
//			}
//		}
//		return defaultValue, fmt.Errorf("null")
//	}

// @Deprecated
func (s Slice[T]) GetOrDefault(index int, defaultValue T) T {
	if index < 0 || index >= len(s) {
		return defaultValue
	}
	return s[index]
}

// @Deprecated
// s.Filter(tools.UnEqual("should not be this","and that")).First()
func (s Slice[T]) FirstUnequal(v T) T {
	for _, e := range s {
		if e != v {
			return e
		}
	}
	return v
}

func ReverseInPlace[T any](s []T) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i] // 双指针交换元素
	}
}
