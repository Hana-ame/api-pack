package tools

import (
	"encoding/json"
	"fmt"
	"net/url"
	"testing"
)

func TestURLParams(t *testing.T) {
	// 解析原始 URL
	rawURL := "/path?foo=bar"
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		fmt.Println("Error parsing URL:", err)
		return
	}

	// 添加参数
	query := parsedURL.Query()
	query.Set("baz", "qux")             // 添加或更新参数
	parsedURL.RawQuery = query.Encode() // 更新 URL 的查询部分

	// 输出修改后的 URL
	fmt.Println("Updated URL:", parsedURL.String())

}

// func TestAnd(t *testing.T) {
// 	var a *testing.B
// 	aa := And(a, 123)
// 	fmt.Println(aa)
// }

func TestSlice(t *testing.T) {
	var s []int
	var js []byte
	s = nil
	fmt.Printf("%v\n", s) // []
	for k, v := range s { // ok
		fmt.Printf("%v, %v\n", k, v)
	}
	js, _ = json.Marshal(s)
	fmt.Printf("%s\n", js) // null

	s = []int{}
	fmt.Printf("%v\n", s) // []
	for k, v := range s { // ok
		fmt.Printf("%v, %v\n", k, v)
	}
	js, _ = json.Marshal(s)
	fmt.Printf("%s\n", js) // []

}

// func TestPop(t *testing.T) {
// 	is := NewSlice(1, 2, 3)
// 	is.Pop()
// 	fmt.Println(is)
// }
