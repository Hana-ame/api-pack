package tools

import "os"

func NewFuncWrapper[T any](result T, err error) *FuncWrapper[T] {
	return &FuncWrapper[T]{
		result: result,
		err:    err,
	}
}

type FuncWrapper[T any] struct {
	result T
	err    error
}

func (w *FuncWrapper[T]) Then(handler func(result T)) *FuncWrapper[T] {
	if w.err == nil {
		handler(w.result)
	}
	return w
}

func (w *FuncWrapper[T]) Catch(handler func(err error)) *FuncWrapper[T] {
	handler(w.err)
	return w
}

// 用法同 a || b
func Or[T comparable](e ...T) T {
	var defaultValue T
	for _, v := range e {
		if v == defaultValue {
			continue
		} else {
			return v
		}
	}
	return defaultValue
}

// 检查是否为空值
func IsDefaultValue[T comparable](e T) bool {
	var defaultValue T
	return e == defaultValue
}

func Ternary[T any](condition bool, a, b T) T {
	if condition {
		return a
	}
	return b
}

// 是不是用不了。
// func And[T any](a any, d T) T {
// 	if a == nil {
// 		return d
// 	}
// 	return a.(T)
// }

type result[T any] struct {
	result T
	err    error
	// defaultResult T
	async bool
}

// catch一个单输出+error的function，并且
// 返回一个result，result有几个方法，可以在一行里面处理
func Match[T any](r T, err error) *result[T] {
	return &result[T]{
		result: r,
		err:    err,
	}
}

func (r *result[T]) GetOrDefault(defaultValue T) T {
	if r.err != nil {
		return defaultValue
	}
	return r.result
}

func (r *result[T]) Result() T {
	return r.result
}

func (r *result[T]) Error() error {
	return r.err
}

// 暂时没想好怎么用
func (r *result[T]) Async() *result[T] {
	r.async = true
	return r
}

func (r *result[T]) Then(f func(result T) error) *result[T] {
	if r.err == nil {
		r.err = f(r.result)
	}
	return r
}

func (r *result[T]) Catch(f func(e error) error) error {
	if r.err == nil {
		return nil
	}
	return f(r.err)
}

// func (r *result[T]) Then(f func(v T) (E, error)) (E, error) {
// 	if r.err == nil {
// 		return f(r.result)
// 	}
// 	return r.err
// }

func HasEnv(key string) bool {
	s, ok := os.LookupEnv(key)
	return ok && s != ""
}

func Unpack[T any](e *T) T {
	if e == nil {
		var zero T
		return zero
	}
	return *e
}
