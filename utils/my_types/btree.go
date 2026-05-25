package mytypes

type TreeNode[T any] struct {
	value T
	left  *TreeNode[T]
	right *TreeNode[T]
}
