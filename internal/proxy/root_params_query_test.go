package proxy

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestRootProxyHandlerParamsQueryExact 验收：
// 把原始 URL 的全部 searchParams 放进 params.query 后，代理实际发往上游的
// path + query 必须与原始 URL 逐字节一致（保留参数顺序、重复键、转义）。
func TestRootProxyHandlerParamsQueryExact(t *testing.T) {
	gin.SetMode(gin.TestMode)

	gotCh := make(chan string, 8)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCh <- r.URL.RequestURI()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL) // http://127.0.0.1:PORT
	if err != nil {
		t.Fatalf("parse upstream url: %v", err)
	}

	cases := []struct {
		name     string
		original string
	}{
		{"用户示例", "https://example.com/path/to/file?param1=123&param2=234"},
		{"无 query", "https://example.com/img/x.jpg"},
		{"单参数", "https://example.com/img/x.jpg?quality=85"},
		{"多参数保持顺序", "https://example.com/img/x.jpg?b=2&a=1&c=3"},
		{"已转义", "https://example.com/img/x.jpg?q=a%20b&x=%2Fslash&empty="},
		{"重复键", "https://example.com/img/x.jpg?t=1&t=2"},
		{"裸键", "https://example.com/img/x.jpg?flag"},
		{"带子路径", "https://example.com/a/b/c.png?w=250&h=250&fmt=webp"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			orig, err := url.Parse(tc.original)
			if err != nil {
				t.Fatalf("parse original: %v", err)
			}

			params := ProxyParams{
				Host:   upstreamURL.Host,
				Scheme: upstreamURL.Scheme,
				Query:  orig.RawQuery,
			}
			jsonBytes, err := json.Marshal(params)
			if err != nil {
				t.Fatalf("marshal params: %v", err)
			}
			b64 := base64.RawURLEncoding.EncodeToString(jsonBytes)

			// 代理 URL 上只允许出现一个 params=，其余原始参数全部在 params 内
			target := orig.Path + "?params=" + b64
			if strings.Count(target, "params=") != 1 {
				t.Fatalf("proxy URL must contain exactly one params=, got %q", target)
			}

			req := httptest.NewRequest(http.MethodGet, target, nil)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			RootProxyHandler(c)

			if w.Code != http.StatusOK {
				t.Fatalf("proxy returned %d: %s", w.Code, w.Body.String())
			}

			select {
			case got := <-gotCh:
				want := orig.RequestURI() // path + "?" + RawQuery
				if got != want {
					t.Errorf("上游收到的 URI 与原始 URL 不一致\n got: %s\nwant: %s", got, want)
				}
			default:
				t.Fatalf("upstream 未被访问")
			}
		})
	}
}

// TestProxyParamsJSONKeepsQueryField 确认 query 字段可被 base64 JSON 往返解析。
func TestProxyParamsJSONKeepsQueryField(t *testing.T) {
	in := ProxyParams{Host: "example.com", Scheme: "https", Query: "b=2&a=1&q=a%20b"}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	b64 := base64.RawURLEncoding.EncodeToString(b)
	decoded, err := base64.RawURLEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	var out ProxyParams
	if err := json.Unmarshal(decoded, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Query != in.Query {
		t.Errorf("query round-trip mismatch: got %q want %q", out.Query, in.Query)
	}
}
