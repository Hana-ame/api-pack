package tools

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/Hana-ame/api-pack/tools/orderedmap"
)

func TestExactg(t *testing.T) {
	// 假设有以下嵌套结构：
	jsonString := `{
  "user": {
    "id": 123,
    "profile": {
      "name": "Alice"
    }
  }
}`

	o := orderedmap.New()
	json.Unmarshal([]byte(jsonString), &o)

	// 提取嵌套值
	id, err := Extract[float64](o, "user", "id") // 123
	fmt.Println(id, err)
	name, err := Extract[string](o, "user", "profile", "name") // "Alice"
	fmt.Println(name, err)

	// 错误处理
	_, err = Extract[string](o, "user", "email") // 错误: 键不存在: 'email'
	fmt.Println(err)
	_, err = Extract[int](o, "user", "profile") // 错误: 类型不匹配 (期望 int 但得到 *Orderedo)
	fmt.Println(err)

}
