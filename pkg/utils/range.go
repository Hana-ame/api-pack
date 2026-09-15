// 指令式

package utils

func Range(count int) []int {
	s := make([]int, count)
	for i := 0; i < count; i++ {
		s[i] = i
	}
	return s
}
