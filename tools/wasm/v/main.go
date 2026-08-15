// 从url生成一个v
package v

import (
	"crypto/sha256"
	"fmt"
	"net/url"
)

//go:wasmexport v
func V(s string) int32 {
	parsedURL, err := url.Parse(s)
	if err != nil {
		return 0
	}
	// 获取查询参数并删除键为 "v" 的项
	query := parsedURL.Query()
	query.Del("v") // 删除参数 "v"

	// 更新 URL 的查询参数
	parsedURL.RawQuery = query.Encode()

	hash := sha256.Sum256([]byte(parsedURL.Path))

	var result int32
	result = (int32(hash[0]) << 0) |
		(int32(hash[1]) << 8 * 1) |
		(int32(hash[3]) << 8 * 2)

	return result

}

func main() {
	fmt.Println(V("https://proxy.moonchan.xyz/?param1=1&v=2&c=3"))
	fmt.Println(V("https://proxy.moonchan.xyz/?par2am1=1&v=2&c=3"))
	fmt.Println(V("https://proxy.moonchan.xyz/?pa5ram1=1&v=2&c=3"))
	fmt.Println(V("https://proxy.moonchan.xyz/?par3am1=1&v=2&c=3"))
	fmt.Println(V("https://proxy.moonchan.xyz/123?param1=1&v=2&c=3"))
	fmt.Println(V("https://proxy.moonchan.xyz/34?par2am1=1&v=2&c=3"))
	fmt.Println(V("https://proxy.moonchan.xyz/5?pa5ram1=1&v=2&c=3"))
	fmt.Println(V("https://proxy.moonchan.xyz/6?par3am1=1&v=2&c=3"))

	fmt.Println(V("https://proxy.moonchan.xyz/211232?param1=1&v=2&c=3"))
	fmt.Println(V("https://proxy.moonchan.xyz/12312?par2am1=1&v=2&c=3"))
	fmt.Println(V("https://proxy.moonchan.xyz/342?pa5ram1=1&v=2&c=3"))
	fmt.Println(V("https://proxy.moonchan.xyz/5555?par3am1=1&v=2&c=3"))
	fmt.Println(V("https://proxy.moonchan.xyz/153423?param1=1&v=2&c=3"))
	fmt.Println(V("https://proxy.moonchan.xyz/32342344?par2am1=1&v=2&c=3"))
	fmt.Println(V("https://proxy.moonchan.xyz/523423?pa5ram1=1&v=2&c=3"))
	fmt.Println(V("https://proxy.moonchan.xyz/1212216?par3am1=1&v=2&c=3"))

}
