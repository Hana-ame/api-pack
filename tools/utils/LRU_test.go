package tools

import (
	"fmt"
	"testing"
)

func TestLRUmain(t *testing.T) {
	// 创建一个 LRUCache，键类型为 string，值类型为 int
	lru := NewLRUCache[string, int](2)

	lru.Put("one", 1)
	lru.Put("two", 2)
	fmt.Println(lru.Get("one"))   // 输出 1
	lru.Put("three", 3)           // 该操作将会移除 "two"
	fmt.Println(lru.Get("two"))   // 输出 0，表示未找到
	lru.Put("four", 4)            // 该操作将会移除 "one"
	fmt.Println(lru.Get("one"))   // 输出 0，表示未找到
	fmt.Println(lru.Get("three")) // 输出 3
	fmt.Println(lru.Get("four"))  // 输出 4
}
