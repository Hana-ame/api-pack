package exhentai

import (
	"bytes"
	"strings"
)

func containsPrefix(prefixSlice []string, v string) bool {
	for _, prefix := range prefixSlice {
		if strings.HasPrefix(v, prefix) {
			return true
		}
	}
	return false
}

var (
	oldPrefix = []byte("https://s.exhentai.org/")
	newPrefix = []byte("https://proxy.moonchan.xyz/")
	newSuffix = []byte("?proxy_host=ehgt.org")
)

func ReplaceManual(data []byte) []byte {
	idx := bytes.Index(data, oldPrefix)
	if idx == -1 {
		return data
	}
	// 预估容量：原长度 + 额外增加的长度（假设平均有 2-3 个替换）
	// 使用 Buffer 避免多次小规模内存分配
	var result bytes.Buffer
	result.Grow(len(data) + 2048)

	lastPos := 0
	for {
		// 1. 在剩余部分寻找前缀位置
		idx := bytes.Index(data[lastPos:], oldPrefix)
		if idx == -1 {
			// 找不到更多匹配了，把剩下的数据全部追加进去
			result.Write(data[lastPos:])
			break
		}

		// 绝对坐标 = 当前偏移量 + 相对位置
		matchStart := lastPos + idx

		// 2. 将“匹配点之前”的所有数据原封不动写入
		result.Write(data[lastPos:matchStart])

		// 3. 确定 path 的范围
		pathStart := matchStart + len(oldPrefix)
		pathEnd := pathStart
		for pathEnd < len(data) {
			b := data[pathEnd]
			// 停止条件：遇到引号、空格、标签结尾、或者 CSS 中的括号
			if b == '"' || b == '\'' || b == ' ' || b == '>' || b == ')' {
				break
			}
			pathEnd++
		}

		// 4. 写入新的 URL 结构
		result.Write(newPrefix)               // 写入 https://proxy.moonchan.xyz/
		result.Write(data[pathStart:pathEnd]) // 写入 原本的 path
		result.Write(newSuffix)               // 写入 ?proxy_host=ehgt.org

		// 5. 更新偏移量，从路径结束位置继续往下找
		lastPos = pathEnd
	}

	return result.Bytes()
}
