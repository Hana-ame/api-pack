// 26.01.11
// 适配myfetchv2
// 26.01.31
// localTCPAddrFromEnv() 本地地址出错

package proxies

import (
	"bytes"
	"crypto/tls"
	"os"
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

func localTCPAddrFromEnv() *net.TCPAddr {
	if ipStr := os.Getenv("LOCAL_IP"); ipStr != "" {
		if ip := net.ParseIP(ipStr); ip != nil {
			return &net.TCPAddr{IP: ip}
		}
	}
	return nil
}

func NyaaProxy() {

	var client *http.Client = func() *http.Client {
		jar, _ := cookiejar.New(nil)
		u, _ := url.Parse("nyaa.si")
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

		host := "nyaa.si"

		header := tools.NewHeader(c.Request.Header)

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
		data = bytes.ReplaceAll(data, []byte(`a.magsrv.com`), []byte(`localhost`))
		data = bytes.ReplaceAll(data, []byte(`cdn.tsyndicate.com`), []byte(`localhost`))
		data = bytes.ReplaceAll(data, []byte(`<div id="dd4ce992-766a-4df0-a01d-86f13e43fd61"></div>`), []byte{})
		data = bytes.ReplaceAll(data, []byte(`<div id="e7a3ddb6-efae-4f74-a719-607fdf4fa1a1"></div>`), []byte{})

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
		c.Writer.Header().Del("X-Forward-For")
		c.Writer.Header().Del("X-Forwarded-Proto")

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

	r.Run("127.25.23.4:8080")

}