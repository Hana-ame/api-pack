package mytypes

type ICompareable[T any] interface {
	CompareTo(other T) int // 返回: -1 (小于), 0 (等于), 1 (大于)
}
type ICompareableAlt[T any] interface {
	EQ(other T) bool
	LT(other T) bool
	LTE(other T) bool
	GT(other T) bool
	GTE(other T) bool
}
