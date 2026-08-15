// 写这个干嘛，没事做。
// 为啥不和js一样用，又为啥要和js一样用。
package streams

import "fmt"

func First[T any](arr []T, filter func(v T) bool) (T, error) {
	for _, v := range arr {
		if filter(v) {
			return v, nil
		}
	}
	var null T
	return null, fmt.Errorf("not found")
}
