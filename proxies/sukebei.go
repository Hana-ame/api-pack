// 26.01.11
// 适配myfetchv2
// 26.01.31
// localTCPAddrFromEnv() 本地地址出错

package proxies

import (
	"bytes"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	myfetch "github.com/Hana-ame/api-pack/tools/my_fetch/v2"
	middleware "github.com/Hana-ame/api-pack/tools/my_gin_middleware"
	tools "github.com/Hana-ame/api-pack/tools/utils"
	"github.com/gin-gonic/gin"
)

func SukebeiProxy() {

	var client *http.Client = func() *http.Client {
		jar, _ := cookiejar.New(nil)
		u, _ := url.Parse("sukebei.nyaa.si")
		jar.SetCookies(u, []*http.Cookie{})
		tr := &http.Transport{
			DialContext: (&net.Dialer{
				LocalAddr: localTCPAddrFromEnv(),
				Timeout:   15 * time.Second,
				KeepAlive: 90 * time.Second,
			}).DialContext,
			MaxIdleConns:        256,
			IdleConnTimeout:     10 * time.Second,
			TLSHandshakeTimeout: 30 * time.Second,
			TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
		}
		return &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
			Transport: tr,
			Jar:       jar,
		}
	}()

	mf := &myfetch.Client{Client: client}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORSMiddleware())

	r.Any("/*any", func(c *gin.Context) {
		c.Header("X-Debug-Request-Host", c.Request.Host)
		c.Header("X-Debug-Header-Host", c.GetHeader("Host"))

		if c.Request.Body != nil {
			defer c.Request.Body.Close()
		}
		path := c.Request.URL.String()

		host := "sukebei.nyaa.si"

		header := tools.NewHeader(c.Request.Header)
		header.Del("Cookie")

		if !slices.Contains([]string{"CN", ""}, c.Request.Header.Get("Cf-Ipcountry")) {
			c.Redirect(http.StatusFound, "https://"+host+path)
			return
		}

		resp, err := mf.Fetch(
			c.Request.Method, "https://"+host+path,
			(header.Header), c.Request.Body)
		if err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}
		defer resp.Body.Close()

		if strings.HasPrefix(path, "/download") {
			for k, vs := range resp.Header {
				if c.Writer.Header().Get(k) != "" {
					continue
				}
				for _, v := range vs {
					c.Writer.Header().Add(k, v)
				}
			}
			c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, map[string]string{
				"X-Host":    host,
				"X-Origin":  header.Get("Origin"),
				"X-Referer": header.Get("Referer"),
				"X-Cookie":  header.Get("Cookie"),
			})
			return
		}

		body, err := myfetch.ResponseToReader(resp)
		if err != nil {
			c.AbortWithError(http.StatusBadGateway, err)
			return
		}

		data, err := io.ReadAll(body)
		if err != nil {
			c.AbortWithError(http.StatusBadGateway, err)
			return
		}

		data = bytes.ReplaceAll(data, []byte("https://"+host), []byte{})
		data = bytes.ReplaceAll(data, []byte(`magsrv.com`), []byte(`localhost`))
		data = bytes.ReplaceAll(data, []byte(`cdn.tsyndicate.com`), []byte(`localhost`))

		c.Writer.Header().Set("Content-Encoding", "identity")
		c.Writer.Header().Set("Content-Length", strconv.Itoa(len(data)))
		for k, vs := range resp.Header {
			if c.Writer.Header().Get(k) != "" {
				continue
			}
			for _, v := range vs {
				c.Writer.Header().Add(k, v)
			}
		}

		if len(data) == 0 {
			c.AbortWithStatus(resp.StatusCode)
			return
		}

		c.DataFromReader(resp.StatusCode, int64(len(data)), resp.Header.Get("Content-Type"), bytes.NewReader(data), map[string]string{
			"X-Host":    host,
			"X-Origin":  header.Get("Origin"),
			"X-Referer": header.Get("Referer"),
			"X-Cookie":  header.Get("Cookie"),
		})

	})

	r.Run("127.25.23.5:8080")

}
