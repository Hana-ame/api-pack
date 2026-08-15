package curl

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	tools "github.com/Hana-ame/api-pack/tools/utils"
	"github.com/gin-gonic/gin"
)

func TestSlice(t *testing.T) {
	s := "HTTP/2 302 "
	// b := []byte(s)
	ss := strings.Split(s, " ")
	fmt.Println(ss)
	ts := tools.Slice[string](ss)
	fmt.Println(ts)
	cs := ts.GetOrDefault(1, "0")
	fmt.Println(cs)

}

func TestCurl111(t *testing.T) {
	code, _, _, err := curl("https://google.com", "-x", "")
	fmt.Println(code, err)
}

// 目的未达成。
func TestCurl1111(t *testing.T) {
	// host := c.Query("host")
	host := "https://wx1.sinaimg.cn/"
	// host = "google.com"
	// u, err := url.Parse(host)
	code, _, body, err := Curl("GET", "myfetch/250818", nil, "", host, nil, "-x", "", "--connect-timeout", "4")

	fmt.Println(string(body))

	fmt.Println(gin.H{
		"code": code,
		"err":  err,
	})

}

func TestCurl(t *testing.T) {
	code, headers, body, err := curl("https://google.com")
	fmt.Println(err)
	fmt.Println(err)

	for _, header := range headers {
		fmt.Println(header.String())
	}
	fmt.Println(err)
	fmt.Println(err)
	fmt.Println(err)
	fmt.Println(string(body))
	fmt.Println(err)
	fmt.Println(err)
	fmt.Println(code)
	fmt.Println(err)
	fmt.Println(err)

}

func TestCurl2(t *testing.T) {
	headers := Headers{
		&Header{
			Key:   "21",
			Value: "21",
		},
		&Header{
			Key:   "121",
			Value: "212",
		},
	}
	cookie := ""
	code, rh, body, err := Curl("GET", "", headers, cookie, "https://getip.moonchan.xyz/echo", nil)
	fmt.Println(rh)
	fmt.Println(err)
	fmt.Println(string(body))
	fmt.Println(code)
}

func TestCurl3(t *testing.T) {
	headers := Headers{
		&Header{
			Key:   "21",
			Value: "21",
		},
		&Header{
			Key:   "121",
			Value: "212",
		},
	}
	cookie := ""
	code, rh, body, err := Curl("GET", "", headers, cookie, "https://getip.moonchan.xyz/echo", nil, "-o", "result.txt")
	// 加了result。txt所以不显示的。2
	fmt.Println(rh)
	fmt.Println(err)
	fmt.Println(string(body))
	fmt.Println(code)
}

func TestXxx(t *testing.T) {
	var stderr bytes.Buffer
	// 定义要执行的curl命令
	cmd := exec.Command("curl", "-s", "-w", "%{http_code}", "https://1google.com") // 使用一个无效的URL来模拟错误

	// 将stderr设置为我们创建的Buffer
	cmd.Stderr = &stderr

	// 执行命令并获取输出
	output, err := cmd.Output()
	// if err != nil {
	// 如果有错误，打印错误信息
	fmt.Println("错误:", err)
	fmt.Println("标准错误输出:", stderr.String())
	// return
	// }

	// 打印正常输出
	fmt.Println(string(output))

}

func TestA(t *testing.T) {
	a := func(s ...string) {
		fmt.Println(s)      // []
		fmt.Println(len(s)) // 0
	}
	a()
}
