// @241015

package curl

import (
	"bytes"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"

	tools "github.com/Hana-ame/api-pack/tools/utils"
)

type Header struct {
	Key   string
	Value string
}

func (h *Header) String() string {
	return h.Key + ": " + h.Value
}

type Headers []*Header

func (h Headers) Get(key string) string {
	for _, header := range h {
		// if strings.ToLower(header.Key) == strings.ToLower(key) {
		if strings.EqualFold(header.Key, key) {
			return header.Value
		}
	}
	return ""
}
func (h Headers) Has(key string) bool {
	for _, header := range h {
		if strings.EqualFold(header.Key, key) {
			return true
		}
	}
	return false
}
func (h Headers) LoadFromHttpHeader(headers http.Header) Headers {
	for k, v := range headers {
		for _, vv := range v {
			h = append(h, &Header{k, vv})
		}
	}
	return h
}
func (h Headers) DumpToHttpHeader() http.Header {
	headers := make(http.Header)
	for _, v := range h {
		headers.Add(v.Key, v.Value)
	}
	return headers
}

func (h Headers) LoadFromText(headers string) Headers {
	for _, header := range strings.Split(headers, "\r\n") {
		arr := strings.Split(header, ": ")
		if len(arr) != 2 {
			continue
		}
		h = append(h, &Header{arr[0], arr[1]})
	}
	return h
}

func (h Headers) DumpToText() string {
	text := ""
	for _, header := range h {
		text += header.String()
		text += "\r\n"
	}
	return strings.TrimSuffix(text, "\r\n")
}

func Get(agent string, headers Headers, cookie string, url string, argv ...string) (statusCode int, respHeaders Headers, body []byte, err error) {
	return Curl(http.MethodGet, agent, headers, cookie, url, nil, argv...)
}
func Post(agent string, headers Headers, cookie string, url string, requestBody []byte, argv ...string) (statusCode int, respHeaders Headers, body []byte, err error) {
	return Curl(http.MethodPost, agent, headers, cookie, url, requestBody, argv...)
}

func Curl(method string, agent string, requestHeaders Headers, cookie string, url string, requestBody []byte, argv ...string) (statusCode int, respHeaders Headers, body []byte, err error) {
	argv = append(argv, "-X", method)
	if agent != "" {
		argv = append(argv, "-A", agent)
	}

	// if bodyReader != nil {
	// 	body, err = io.ReadAll(bodyReader)
	// 	if err != nil {
	// 		return
	// 	}
	// }
	for _, header := range requestHeaders {
		argv = append(argv, "-H", header.String())
	}
	if cookie != "" {
		if strings.HasSuffix(cookie, ".txt") {
			argv = append(argv, "-c", cookie)
		} else {
			argv = append(argv, "-b", cookie)
		}
	}

	// 添加请求体
	if len(requestBody) > 0 {
		argv = append(argv, "-d", string(requestBody))
	}

	argv = append(argv, url)

	return curl(argv...)
}

func curl(argv ...string) (statusCode int, headers Headers, body []byte, err error) {
	var stderr bytes.Buffer
	argv = append([]string{"-i", "-s"}, argv...)

	cmd := exec.Command("curl", argv...)

	// 将stderr设置为我们创建的Buffer
	cmd.Stderr = &stderr

	// 执行命令并获取输出
	data, err := cmd.Output()
	if err != nil {
		return
	}
	// statusCode, err = strconv.Atoi(string(body[len(body)-3:]))
	// if err != nil {
	// 	return
	// }
	errstr := stderr.String()
	if len(errstr) > 0 {
		err = fmt.Errorf("%s", errstr)
		if err != nil {
			return
		}
	}
	var headSlice []byte
	headSlice, body, err = tools.Seprate([]byte("\r\n\r\n"), data)
	if err != nil {
		return
	}
	if bytes.Equal(headSlice, []byte("HTTP/1.1 200 Connection established")) {
		headSlice, body, err = tools.Seprate([]byte("\r\n\r\n"), body)
		if err != nil {
			return
		}
	}
	var headerSlice, codeSlice []byte
	codeSlice, headerSlice, err = tools.Seprate([]byte("\r\n"), headSlice)
	if err != nil {
		return
	}
	statusCode, err = strconv.Atoi(
		tools.Slice[string](strings.Split(string(codeSlice), " ")).GetOrDefault(1, "0"))

	if err != nil {
		return
	}
	headers = make(Headers, 0).LoadFromText(string(headerSlice))

	// 打印正常输出
	return statusCode, headers, body, err

}
