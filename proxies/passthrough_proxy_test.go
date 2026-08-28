package proxies

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// TestPassthroughProxyPathNormalizesToV1 —— 回归测试: 上游地址的 /v1 不能丢。
//
// 背景: GenericProxyHandler 把 Endpoint 拼在客户端路径前面, 自己不改路径
// (见 sensenova 的配置, Endpoint=https://token.sensenova.cn, 页面用 /v1/images/generations)。
// 所以 SCNET_ENDPOINT 默认配到 /v1 段上, 裸路径 /chat/completions 必须由代理补齐,
// 否则打到 .../api/llm/chat/completions 直接 404。
// 同时验证"透传"语义: Authorization 原样转发, 不做任何 key 注入。
func TestPassthroughProxyPathNormalizesToV1(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var gotPath string
	var gotAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	r := gin.New()
	r.Any("/*any", PassthroughProxy(PassthroughConfig{}, upstream.URL+"/api/llm/v1"))

	cases := []struct {
		in   string
		want string
	}{
		{"/chat/completions", "/api/llm/v1/chat/completions"},
		{"/v1/chat/completions", "/api/llm/v1/chat/completions"},
		{"/models", "/api/llm/v1/models"},
		{"/v1/models", "/api/llm/v1/models"},
	}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, tc.in, nil)
		req.Header.Set("Authorization", "Bearer sk-client-key")
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("%s: 状态码 = %d, want 200", tc.in, w.Code)
		}
		if gotPath != tc.want {
			t.Errorf("%s: 上游收到 %s, want %s", tc.in, gotPath, tc.want)
		}
		if gotAuth != "Bearer sk-client-key" {
			t.Errorf("%s: Authorization = %q, want 原样透传", tc.in, gotAuth)
		}
	}
}

// TestPassthroughProxyBaseWithoutV1 —— base 不带 /v1 时不要硬塞。
// base=.../api/llm 时上游本来就没有 /v1 段, 裸路径就该打 .../api/llm/chat/completions;
// 代理只负责"base 带 /v1 时不漏", 不负责替上游凭空造路径段。
func TestPassthroughProxyBaseWithoutV1(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	r := gin.New()
	r.Any("/*any", PassthroughProxy(PassthroughConfig{}, upstream.URL+"/api/llm"))

	for _, path := range []string{"/chat/completions", "/v1/chat/completions"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, nil))
		want := "/api/llm/chat/completions"
		if strings.HasPrefix(path, "/v1") {
			// base 不带 /v1 时, 客户端自带 /v1 前缀就原样保留。
			want = "/api/llm/v1/chat/completions"
		}
		if gotPath != want {
			t.Errorf("base 不带 /v1, 入口 %s: 上游收到 %s, want %s", path, gotPath, want)
		}
	}
}

// TestPassthroughProxyNoKeyInjection —— 透传代理必须不碰 Authorization。
// GenericProxyHandler 会按 FreeModels/OverrideAuth 改写 Authorization,
// 这里刻意验证 PassthroughProxy 在两种情形下都保持原样。
func TestPassthroughProxyNoKeyInjection(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var gotAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer upstream.Close()

	r := gin.New()
	r.Any("/*any", PassthroughProxy(PassthroughConfig{}, upstream.URL+"/v1"))

	for _, auth := range []string{"Bearer sk-my-own-key", ""} {
		gotAuth = "<unset>"
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/chat/completions", nil)
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		r.ServeHTTP(w, req)

		want := ""
		if auth != "" {
			want = auth
		}
		if gotAuth != want {
			t.Errorf("输入 Authorization=%q: 上游收到 %q, want 原样", auth, gotAuth)
		}
	}
}

// TestPassthroughProxyClientTimeout —— 透传 client 不能继承 DefaultClient 的 30s 超时。
// 背景: main() 把 http.DefaultClient 换成带 Timeout: 30s 的 client; LLM 生成
// 可能几十秒, 直接用它会被掐断。SCNET_TIMEOUT 留空时应不设超时, 填秒数时应生效。
func TestPassthroughProxyClientTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	rNoTimeout := gin.New()
	rNoTimeout.Any("/*any", PassthroughProxy(PassthroughConfig{}, upstream.URL+"/v1"))

	rWithTimeout := gin.New()
	rWithTimeout.Any("/*any", PassthroughProxy(PassthroughConfig{Timeout: 10 * time.Millisecond}, upstream.URL+"/v1"))

	// 不设超时: 150ms 的上游延迟必须成功
	w := httptest.NewRecorder()
	rNoTimeout.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/chat/completions", nil))
	if w.Code != http.StatusOK {
		t.Errorf("Timeout=0: 状态码 = %d, want 200", w.Code)
	}

	// 设 10ms: 150ms 的上游必须超时失败 (502)
	w = httptest.NewRecorder()
	rWithTimeout.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/chat/completions", nil))
	if w.Code != http.StatusBadGateway {
		t.Errorf("Timeout=10ms: 状态码 = %d, want 502", w.Code)
	}
}
