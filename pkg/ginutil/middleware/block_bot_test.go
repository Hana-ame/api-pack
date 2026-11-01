package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// newBotTestEngine 构造一个仅挂 BlockBotMiddleware + 记录 handler 是否执行的路由。
func newBotTestEngine(handlerCalled *bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(BlockBotMiddleware())
	r.GET("/api/v2/", func(c *gin.Context) {
		*handlerCalled = true
		c.JSON(http.StatusOK, gin.H{"threads": []int{1, 2, 3}})
	})
	return r
}

func performBotRequest(r *gin.Engine, bid, ua string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v2/?bid="+bid, nil)
	req.Header.Set("User-Agent", ua)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// 敏感版块 + 爬虫：必须返回空数据，且 handler 不执行。
func TestBlockBotMiddlewareSensitiveBidWithBot(t *testing.T) {
	for _, bid := range []string{"46", "48", "103"} {
		for _, ua := range []string{
			"Googlebot/2.1 (+http://www.google.com/bot.html)",
			"Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)",
			"Mozilla/5.0 (compatible; Baiduspider/2.0; +http://www.baidu.com/search/spider.html)",
			"Mozilla/5.0 (compatible; YandexBot/3.0; +http://yandex.com/bots)",
		} {
			handlerCalled := false
			r := newBotTestEngine(&handlerCalled)
			w := performBotRequest(r, bid, ua)
			if w.Code != http.StatusOK {
				t.Fatalf("bid=%s ua=%q: status=%d, want 200", bid, ua, w.Code)
			}
			var body map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("bid=%s ua=%q: decode body: %v (body=%q)", bid, ua, err, w.Body.String())
			}
			if body["message"] != "Empty" || body["data"] != nil {
				t.Fatalf("bid=%s ua=%q: body=%v, want empty payload", bid, ua, body)
			}
			if handlerCalled {
				t.Fatalf("bid=%s ua=%q: handler must not run for blocked bot", bid, ua)
			}
		}
	}
}

// 敏感版块 + 正常浏览器：必须放行。
func TestBlockBotMiddlewareSensitiveBidWithBrowser(t *testing.T) {
	handlerCalled := false
	r := newBotTestEngine(&handlerCalled)
	w := performBotRequest(r, "46", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/130.0 Safari/537.36")
	if w.Code != http.StatusOK || !handlerCalled {
		t.Fatalf("status=%d handlerCalled=%v, want handler executed", w.Code, handlerCalled)
	}
}

// 非敏感版块 + 爬虫：必须放行。
func TestBlockBotMiddlewareNonSensitiveBidWithBot(t *testing.T) {
	handlerCalled := false
	r := newBotTestEngine(&handlerCalled)
	w := performBotRequest(r, "1", "Googlebot/2.1 (+http://www.google.com/bot.html)")
	if w.Code != http.StatusOK || !handlerCalled {
		t.Fatalf("status=%d handlerCalled=%v, want handler executed", w.Code, handlerCalled)
	}
}
