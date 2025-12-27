// 在 .env 中设置 EXHENTAI_PROXY_COOKIE 项目以更新Cookie

package main

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Hana-ame/api-pack/tools/debug"
	myfetch "github.com/Hana-ame/api-pack/tools/my_fetch"
	"github.com/Hana-ame/api-pack/tools/my_fetch/my_if"
	middleware "github.com/Hana-ame/api-pack/tools/my_gin_middleware"
	tools "github.com/Hana-ame/api-pack/tools/utils"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"golang.org/x/net/html"

	"github.com/antchfx/htmlquery"
)

// toolkik: xpath

const InnerText string = "INNER_TEXT"

func findOneAndSelectAttr(top *html.Node, expr string, name string) (v string, err error) {
	elem := htmlquery.FindOne(top, expr)
	if elem == nil {
		err = errors.New(expr + ":" + name + "is null")
		return
	}
	if name == InnerText {
		v = htmlquery.InnerText(elem)
	} else {
		v = htmlquery.SelectAttr(elem, name)
	}
	return
}

func findAll(top *html.Node, expr, name string) (v []string) {
	elemArray := htmlquery.Find(top, expr)
	v = make([]string, len(elemArray))
	for i, e := range elemArray {
		if name == InnerText {
			v[i] = htmlquery.InnerText(e)
		} else {
			v[i] = htmlquery.SelectAttr(e, name)
		}
	}
	return
}

// end toolkik: xpath

// 大概可以，没试过
//
//	var jar = func() *cookiejar.Jar {
//		jar, _ := cookiejar.New(nil)
//		u, _ := url.Parse("exhentai.org")
//		jar.SetCookies(u, []*http.Cookie{{
//			Name:  "ipb_member_id",
//			Value: "5698562",
//			Path:  "/",
//		}, {
//			Name:  "ipb_pass_hash",
//			Value: "154e574fd19294c32f905fe187cbdad1",
//			Path:  "/",
//		}, {
//			Name:  "igneous",
//			Value: "5eevdxac75hpx71cv",
//			Path:  "/",
//		}})
//		return jar
//	}()
var jar *cookiejar.Jar = nil

var last uint64

// 就是这个
func ExhProxy() {
	godotenv.Load(".env")

	// debug.LogLevel = debug.Fatal

	prefix := tools.NewSlice(
		os.Getenv("EXHENTAI_PROXY_PREFIX"),
		"2001:470:c:6c:",
	).FirstUnequal("")

	ips := []net.IP{my_if.NewAddr(prefix), my_if.NewAddr(prefix)}
	ipidx := 0

	my_if.AddAddr(ips[ipidx].String())
	cp := myfetch.NewClientPool([]*http.Client{
		myfetch.NewV6Client(ips[ipidx], jar),
	})

	// cp = nil // debug
	mf := myfetch.NewFetcher(nil, cp)

	// gin
	r := gin.Default()

	// 设置block
	// r.Use(middleware.BlockMiddleware()) // 改到下面

	// 设置 CORS 头
	// r.Use(gzip.Gzip(gzip.DefaultCompression))                                                                                                                                                        nnnn
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.ProxyMiddleware())

	// 定义一个简单的 GET 路由
	r.Any("/*any", func(c *gin.Context) {
		// if strings.HasPrefix(c.Request.URL.String(), "/still-alive") {
		// 	last = uint64(time.Now().Unix())
		// 	c.String(200, "%d", last)
		// 	c.Abort()
		// 	return
		// }
	}, func(c *gin.Context) {
		// 送到新域名, 若 迁移(1/2)
		if c.Request.Host == "ex.nmbyd1.top" {
			// 获取请求中的 Cookie
			href, err := url.Parse(c.Request.URL.String())
			if err != nil {
				c.Header("X-Error", c.Request.URL.String())
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}
			href.Host = "ex.nmbyd2.top"
			href.Scheme = "https"
			c.Header("X-Location", href.String()) // 在那之前先在X-Location上看一看
			// 闹钟
			if time.Now().Before(time.Date(2025, 4, 10, 0, 0, 0, 0, time.Local)) {
				return
			}
			c.Redirect(http.StatusMovedPermanently, href.String())
			c.Abort()
			return
		}

	}, func(c *gin.Context) {
		// 封禁列表
		// archive, fullimg, uconfig
		// 国内且pass != pass

		if strings.HasPrefix(c.Request.URL.String(), "/api") {
			c.AbortWithStatus(http.StatusGone)
			return
		}

		if strings.HasPrefix(c.Request.URL.String(), "/fullimg") {
			// 如果 Cookie 包含 pass=pass，直接继续处理请求
			if cookie, err := c.Cookie("pass"); err != nil || cookie == "pass" {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
		}

		if strings.HasPrefix(c.Request.URL.String(), "/archiver.php") {
			// if last+60 < uint64(time.Now().Unix()) {
			// 	c.String(200, "关机节电中")
			// 	c.Abort()
			// 	return
			// }
			// 如果 Cookie 包含 pass=pass，直接继续处理请求
			c.Redirect(301, "https://exhentai.org"+c.Request.URL.String())
			c.Abort()
			return
		}

		// if strings.HasPrefix(c.Request.URL.String(), "/static/") {
		// 	c.Redirect(http.StatusFound, "https://moonchan.xyz"+c.Request.URL.String())
		// 	c.Abort()
		// 	return
		// }

		// 遗留问题, image这个path重定向到用param的请求
		if strings.HasPrefix(c.Request.URL.String(), "/image/") {
			path := strings.TrimPrefix(c.Request.URL.String(), "/image")
			parsedURL, err := url.Parse(path)
			if err != nil {
				c.Header("X-Error", parsedURL.String())
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}
			query := parsedURL.Query()
			query.Set("redirect_to", "image")
			parsedURL.RawQuery = query.Encode()

			c.Redirect(http.StatusMovedPermanently, parsedURL.String())
			c.Abort()
			return
		}

		if strings.HasPrefix(c.Request.URL.String(), "/uconfig.php") {
			// 修改为设置页面
			const href = "https://config.810114.xyz/exhentai/settings.html"
			resp, err := myfetch.Fetch(
				c.Request.Method, href,
				nil, c.Request.Body)
			if err != nil {
				c.Header("X-Error", err.Error())
				c.AbortWithError(http.StatusBadGateway, err)
				return
			}
			defer resp.Body.Close()

			c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, map[string]string{
				"X-Href": href,
			})
			c.Abort()
			return
		}

		if strings.HasPrefix(c.Request.URL.String(), "/mytags") {
			// 如果 Cookie 包含 pass=pass，直接继续处理请求
			if cookie, err := c.Cookie("pass"); err != nil || cookie == "pass" {
				c.String(http.StatusOK, "全局影响项，不样改。在search里面用 girl -[不想要的tag] 方式平替（只有-tag排除时无效,前面需要加个girl）")
				c.Abort()
				return
			}

		}

	}, func(c *gin.Context) {
		// 单纯封禁非中国

		// redirect_to image 不阻止.
		if c.Query("redirect_to") != "" {
			return
		}

		// 适配自家阅读器
		refererHost := tools.Match(url.Parse(c.Request.Referer())).GetOrDefault(c.Request.URL).Host
		if refererHost == "page.moonchan.xyz" ||
			strings.Contains(refererHost, "nmbyd") ||
			strings.Contains(refererHost, "moonchan") {
			return
		}

		// 获取请求中的 Cookie
		cookie, err := c.Cookie("pass")

		// 如果 Cookie 包含 pass=pass，不阻止
		if err == nil && cookie == "pass" {
			return
		}

		// 放行标头存在zh的
		if strings.Contains(c.GetHeader("accept-language"), "zh") {
			return
		}

		// 如果不在 "CN", "" 中的任意一个。则 block, 防止DMCA
		if !slices.Contains([]string{"CN", ""}, c.Request.Header.Get("Cf-Ipcountry")) {
			// c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			// 	"message":      "出于DMCA原因禁止访问, 请关闭代理/翻墙工具",
			// 	"Cf-Ipcountry": c.Request.Header.Get("Cf-Ipcountry"),
			// })
			c.String(http.StatusForbidden, "出于DMCA原因禁止访问, 请关闭代理/翻墙工具再尝试进行访问\n你当前IP属地: %s (必须为CN)\nexhentai镜像：https://ex.moonchan.xyz\nexhentai镜像：https://ex.nmbyd2.top\n反馈请致：https://moonchan.xyz", c.Request.Header.Get("Cf-Ipcountry"))
			c.Abort()
			return
		}

		// }, func(c *gin.Context) { // 用于迁移域名

		//  若 迁移(2/2)
		if c.Request.Host == "ex.nmbyd1.top" {
			// 获取请求中的 Cookie
			cookie, err := c.Cookie("at")
			// 如果 Cookie 包含 pass=pass，不阻止
			if err == nil {
				timestamp := tools.Atoi(cookie, -1)
				if timestamp > 0 && timestamp < int(tools.NewTimeStamp()) { // 如果大于
					return
				}
			}
			cookieValue := strconv.Itoa(int(tools.NewTimeStamp()) + 65536*1000*5)
			c.SetCookie("at", cookieValue, 3600*24*365, "/", "", false, false)
			c.String(http.StatusOK, "为了节约3$预算以及防止一个域名用太久被墙，现已经迁移到：https://ex.nmbyd2.top\nhttps://ex.nmbyd1.top 将在5月2日过期，在这之前你依然能够使用这个域名\n这条警告消息被设置为只会出现在首次访问的5秒内，以防有人看不见\n类似这样的迁移频率大致为10月一次，嫌麻烦可以给我打钱，钱够就续到被墙")
			c.Abort()
			return
		}

		/// 顺序好像不对，不起用了。
		// if r, err := (url.Parse(c.Request.Referer())); err == nil && slices.Contains([]string{"ex.nmbyd1.top", "ex.nmbyd2.top", "ex.nmbyd3.top", "ex.moonchan.xyz"}, r.Host) {
		// 	return
		// }

	}, func(c *gin.Context) {

		// 看看有多少余额用的

		path := c.Request.URL.String()
		if strings.HasPrefix(path, "/exchange.php") || strings.HasPrefix(path, "/home.php") || strings.HasPrefix(path, "/logs.php") {
			header := tools.NewHeader(c.Request.Header)
			header.Set(
				"Cookie",
				tools.NewSlice(
					c.GetHeader("X-Cookie"),
					os.Getenv("EXHENTAI_PROXY_COOKIE"),
					"ipb_member_id=5698562; ipb_pass_hash=154e574fd19294c32f905fe187cbdad1; yay=louder; igneous=5eevdxac75hpx71cv",
				).FirstUnequal(""),
			)
			resp, err := myfetch.Fetch(
				c.Request.Method, "https://e-hentai.org"+path,
				(header.Header), c.Request.Body)
			if err != nil {
				debug.E("why", err.Error())
				c.Header("X-Error", err.Error())
				c.AbortWithError(http.StatusBadGateway, err)
				return
			}
			defer resp.Body.Close()

			for k, vs := range resp.Header {
				if c.Writer.Header().Get(k) != "" {
					continue
				}
				for _, v := range vs {
					c.Writer.Header().Add(k, v)
				}
			}
			c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, map[string]string{
				"X-Path": path,
			})

			c.Abort()
			return
		}
	}, func(c *gin.Context) {

		// 屏蔽php攻击

		path := c.Request.URL.Path
		allowList := []string{"/gallerytorrents.php", "/favorites.php", "/torrents.php", "/gallerypopups.php"}
		for _, allowed := range allowList {
			if strings.HasPrefix(path, allowed) {
				return
			}
		}
		if strings.HasSuffix(path, "php") {
			c.String(http.StatusForbidden, "疑似攻击, 已屏蔽 %s, %s", c.Request.URL.Path, c.Request.URL.String())
			c.Abort()
			return
		}
	}, func(c *gin.Context) {

		// 正牌

		c.Header("X-Debug-Request-Host", c.Request.Host)     // 要设置 Host $http_host
		c.Header("X-Debug-Header-Host", c.GetHeader("Host")) // never

		if c.Request.Body != nil {
			defer c.Request.Body.Close()
		}
		// 读取 URL 参数
		path := c.Request.URL.String()

		host := tools.Or(c.Query("host"), "exhentai.org")
		if strings.HasPrefix(c.Request.URL.String(), "/static/") ||
			strings.HasPrefix(c.Request.URL.String(), "/sw.js") ||
			strings.HasPrefix(c.Request.URL.String(), "/manifast.json") {
			host = "page.moonchan.xyz"
		}

		header := tools.NewHeader(c.Request.Header)
		header.Set("Cookie",
			tools.NewSlice(
				c.GetHeader("X-Cookie"),
				os.Getenv("EXHENTAI_PROXY_COOKIE"),
			).FirstUnequal(""),
		)
		// 如果头带Mobile就OverRide掉.
		if strings.Contains(header.Get("User-Agent"), "Mobile") {
			header.Set("User-Agent", "myfetch/2025.4.14")
		}

		// 修改Referer
		tools.Match(url.Parse(c.Request.Referer())).Then(func(url *url.URL) error {
			url.Scheme = "https"
			url.Host = "exhentai.org"
			header.Set("Referer", url.String())
			return nil
		}).Catch(func(e error) error {
			c.Header("X-Error", e.Error())
			header.Set("Referer", "https://exhentai.org/")
			return nil
		})

		resp, err := mf.Fetch(
			c.Request.Method, "https://"+host+path,
			(header.Header), c.Request.Body)
		if err != nil {
			debug.E("why", err.Error())
			c.Header("X-Error", err.Error())
			c.AbortWithError(http.StatusBadGateway, err)
			return
		}
		defer resp.Body.Close()

		if mf.Count() > 1000 {
			ipidx = (ipidx + 1) % len(ips)
			defer func(ip string) {
				time.Sleep(240 * time.Second)
				my_if.DelAddr(ip)
			}(ips[ipidx].String())
			ips[ipidx] = my_if.NewAddr(prefix)
			my_if.AddAddr(ips[ipidx].String())
			newCp := myfetch.NewClientPool([]*http.Client{myfetch.NewV6Client(ips[ipidx], jar)})
			mf.SetClientPool(newCp)
		}

		// 是 torrent， 是js，就直接返回
		if strings.HasPrefix(path, "/torrent") || strings.HasPrefix(path, "/z/") {
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
			c.Abort()
			return
		}

		// resp 获得 text
		// 需要override https://exhentai.org/s/, https://exhentai.org/g/
		body, err := myfetch.ResponseToReader(resp)
		if err != nil {
			debug.E("why", err.Error())
			c.Header("X-Error", err.Error())
			c.AbortWithError(http.StatusBadGateway, err)
			return
		}

		data, err := io.ReadAll(body)
		if err != nil {
			c.Header("X-Error", err.Error())
			c.AbortWithError(http.StatusBadGateway, err)
			return
		}

		if len(data) == 0 {
			c.AbortWithStatus(resp.StatusCode) // 304 not modified
			tools.PatchHeader(c, resp.Header)
			return
		}

		// data = bytes.ReplaceAll(data, []byte("https://exhentai.org/g/"), []byte("https://"+"ex.nmbyd1.top"+"/g/"))
		// data = bytes.ReplaceAll(data, []byte("https://exhentai.org/s/"), []byte("https://"+"ex.nmbyd1.top"+"/s/"))
		// data = bytes.ReplaceAll(data, []byte("https://exhentai.org/z/"), []byte("https://"+"ex.nmbyd1.top"+"/z/"))
		// data = bytes.ReplaceAll(data, []byte("https://exhentai.org/img/"), []byte("https://"+"ex.nmbyd1.top"+"/img/"))
		data = bytes.ReplaceAll(data, []byte("https://exhentai.org"), []byte{})
		c.Header("X-Debug", c.GetHeader("Host"))
		// data = bytes.ReplaceAll(data, []byte("https://s.exhentai.org"), []byte("https://s-ex.moonchan.xyz"))
		data = bytes.ReplaceAll(data, []byte("https://s.exhentai.org"), []byte("https://ehgt.org"))
		if strings.HasPrefix(path, `/s/`) {
			data = []byte(addWaterFallViewButton(string(data)))
		} else if strings.HasPrefix(path, "/g/") {
			// nothing here
		} else {
			data = addReloadCoverButton(data)
		}
		data = addFloatingIframeAtRightBottom(data)
		if host != "page.moonchan.xyz" {
			// 随同inline chat room, 应该一同加载一些tamper monkey脚本
			data = addInlineChatRoom(data)
		}

		// 获得param，redirect_to=image
		// 没经过类型检查
		if strings.HasPrefix(path, "/s/") && c.Query("redirect_to") == "image" {
			doc, err := htmlquery.Parse(bytes.NewReader(data))
			if err != nil {
				c.Header("X-Error", err.Error())
				c.AbortWithStatus(http.StatusBadGateway)
				return
			}

			image, err := findOneAndSelectAttr(doc, "//img[@id='img']", "src")
			c.Redirect(http.StatusFound, image)
			c.Abort()
			return
		}

		if strings.HasPrefix(path, "/g/") && c.Query("redirect_to") == "json" {
			doc, err := htmlquery.Parse(bytes.NewReader(data))
			if err != nil {
				c.Header("X-Error", err.Error())
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			date, _ := findOneAndSelectAttr(doc, `//*[@id="gdd"]/table/tbody/tr[1]/td[2]`, InnerText)
			// imageArray, _ := findAll(doc, `//a[@]`) // 不做了
			c.JSON(http.StatusOK, map[string]string{
				"date": date,
			})
			c.Abort()
			return
		}

		if strings.HasPrefix(path, "/g/") && c.Query("redirect_to") == "cover" {
			doc, err := htmlquery.Parse(bytes.NewReader(data))
			if err != nil {
				c.Header("X-Error", err.Error())
				c.AbortWithStatus(http.StatusBadGateway)
				return
			}
			hrefArray := findAll(doc, "//a", "href")
			for _, href := range hrefArray {
				if strings.HasPrefix(href, "/s/") {
					parsedURL, err := url.Parse(href)
					if err != nil {
						c.Header("X-Error", parsedURL.String())
						c.AbortWithStatus(http.StatusBadRequest)
						return
					}
					query := parsedURL.Query()
					query.Set("redirect_to", "image")
					parsedURL.RawQuery = query.Encode()

					c.Redirect(http.StatusMovedPermanently, parsedURL.String())
					c.Abort()
					return
				}
			}
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		// 为什么自带的方法这么贵物
		// c.Writer.Header().Set("Content-Encoding", "identity")
		// 和最后的方法到底用哪个。
		// c.Writer.Header().Set("Content-Length", strconv.Itoa(len(data)))
		// c.Header("Content-Length", strconv.Itoa(len(compressedData))) // 长度必须是压缩后的长度

		for k, vs := range resp.Header {
			if c.Writer.Header().Get(k) != "" {
				continue
			}
			for _, v := range vs {
				c.Writer.Header().Add(k, v)
			}
		}
		// c.Header("Content-Type", resp.Header.Get("Content-Type"))

		// 【重点】强制删除 Content-Length，触发 Chunked 传输
		// 这样你就不用算长度，也不会报“多写”
		c.Header("Content-Length", "")

		// 这是在干嘛..
		if slices.Contains([]int{http.StatusNotFound}, resp.StatusCode) {
			c.Redirect(http.StatusTemporaryRedirect, "https://"+c.Request.Host)
			c.Abort()
			return
		}

		// 把你那个 map 里的头搬出来手动设
		c.Header("X-Host", host)
		c.Header("X-Origin", header.Get("Origin"))
		c.Header("X-Referer", header.Get("Referer"))
		c.Header("X-Cookie", header.Get("Cookie"))
		c.Header("X-Debug-Request-Host", c.Request.Host)
		c.Header("X-Debug-Header-Host", c.GetHeader("Host"))

		c.Writer.Header().Set("Content-Encoding", "gzip") // there is no difference with the code above

		c.Status(resp.StatusCode)

		// 6. 接管 Writer 进行流式压缩
		gz := gzip.NewWriter(c.Writer)
		defer gz.Close() // 必须 defer Close，否则 gzip 数据不完整，浏览器会报解压失败

		if _, err := gz.Write(data); err != nil {
			return // 客户端断开了
		}

		// 为什么会报多写。
		// c.DataFromReader(resp.StatusCode, -1, resp.Header.Get("Content-Type"), bytes.NewReader(data), map[string]string{
		// 	"X-Host":               host,
		// 	"X-Origin":             header.Get("Origin"),
		// 	"X-Referer":            header.Get("Referer"),
		// 	"X-Cookie":             header.Get("Cookie"),
		// 	"X-Debug-Request-Host": c.Request.Host,
		// 	"X-Debug-Header-Host":  c.GetHeader("Host"),
		// })

	})

	// 启动服务器
	r.Run("127.25.23.2:8080") // 在 8080 端口启动服务

}

// 要cookie，但是为森么还是不行。
func SProxy() {

	// 使用 gin.New() 创建新的 Gin 实例，避免默认的日志中间件
	r := gin.New()

	// 添加 Recovery 中间件（可选）
	r.Use(gin.Recovery())

	// r := gin.Default()

	// 设置 CORS 头
	r.Use(middleware.CORSMiddleware())

	// 定义一个简单的 GET 路由
	r.Any("/*any", func(c *gin.Context) {
		if c.Request.Body != nil {
			defer c.Request.Body.Close()
		}
		// 读取 URL 参数
		path := c.Request.URL.String()

		if strings.HasPrefix(path, "/api") {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		// if strings.HasPrefix(path, "/fullimg/") {
		// 	c.AbortWithStatus(http.StatusServiceUnavailable)
		// 	return
		// }

		host := "s.exhentai.org"

		header := tools.NewHeader(c.Request.Header)
		header.Set(
			"Cookie",
			tools.NewSlice(
				c.GetHeader("X-Cookie"),
				os.Getenv("EXHENTAI_PROXY_COOKIE"),
			).FirstUnequal(""),
		)

		if c.Request.Method == http.MethodGet {
			resp, err := myfetch.Fetch(
				http.MethodHead, "https://ehgt.org"+path,
				(header.Header), c.Request.Body)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					c.Redirect(http.StatusMovedPermanently, "https://ehgt.org"+path)
					return
				}
			}
		}

		resp, err := myfetch.Fetch(
			c.Request.Method, "https://"+"s.exhentai.org"+path,
			(header.Header), c.Request.Body)
		if err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}
		defer resp.Body.Close()

		// for k, vs := range resp.Header {
		// 	if c.Writer.Header().Get(k) != "" {
		// 		continue
		// 	}
		// 	for _, v := range vs {
		// 		c.Writer.Header().Add(k, v)
		// 	}
		// }
		tools.PatchHeader(c, resp.Header)

		c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, map[string]string{
			"X-Host":    host,
			"X-Origin":  header.Get("Origin"),
			"X-Referer": header.Get("Referer"),
			"X-Cookie":  header.Get("Cookie"),
		})

	})

	// 启动服务器
	r.Run("127.25.23.3:8080") // 在 8080 端口启动服务

}

func addReloadCoverButton(html []byte) []byte {
	html = bytes.Replace(html, []byte("<body>"), []byte(`<body>
	<div style="
	  height: 60px;
	  width: 100px;
	  text-align: center;
	  /* background-color: violet; */
	  position: fixed;
	  right: 20px; 
	  top: 20px;
	  z-index: 99;
	  display: table-cell;
	  vertical-align: middle;
	  /* float: right; */
	">
	  <button id="reload-cover" style="
			width: 100%;    
			height: 100%;
			font-size: x-large;
			display: none;
	  ">
			重新加载封面
	  </button>
	</div>
`), 1)

	html = bytes.Replace(html, []byte("</body>"), []byte(`<script type="text/javascript">
	// 获取所有class为gl3t的元素
	var gl3tElements = document.getElementsByClassName('gl3t');

	if (gl3tElements.length > 0) 
		document.getElementById('reload-cover').style.display = 'block';

	async function execReload() {
		window.stop();

		var gl3tElements = document.getElementsByClassName('gl3t');
		// 遍历每个gl3t元素
		for (var i = 0; i < gl3tElements.length; i++) {
			// 获取当前元素中的所有a标签
			var links = gl3tElements[i].getElementsByTagName('a');
			for (var j = 0; j < links.length; j++) {
				// 获取a标签的href
				var href = links[j].href;
				console.log(links[j]);
				// 获取当前a标签中的img标签
				var imgs = links[j].getElementsByTagName('img');
				for (var k = 0; k < imgs.length; k++) {
					// 修改img的src属性
					imgs[k].src = href + '?redirect_to=cover';
				}
			}
		}
	}
	document.getElementById("reload-cover").addEventListener("click", execReload, false); 

	// if (gl3tElements.length > 0) 
	// 	execReload();

	</script>

</body>`), 1)

	return html
}

func addWaterFallViewButton(html string) string {
	return strings.Replace(html, "<body>", `<body>
	<div style="
	  height: 60px;
	  width: 100px;
	  text-align: center;
	  /* background-color: violet; */
	  position: fixed;
	  right: 20px; 
	  top: 20px;
	  z-index: 99;
	  display: table-cell;
	  vertical-align: middle;
	  /* float: right; */
	">
	  <button id="waterfall" style="
			width: 100%;    
			height: 100%;
			font-size: x-large;
	  ">
			下拉式1
	  </button>
	  <button id="waterfall2" style="
			width: 100%;    
			height: 100%;
			font-size: x-large;
	  ">
			下拉式2
	  </button>
	</div>
	<!-- 新增的左上角按钮、不行要C，还是删了 -->
	<div style="
		height: 60px;
		width: 100px;
		text-align: center;
		position: fixed;
		left: 20px; 
		top: 20px;
		z-index: 99;"
	>
		<button id="originBtn" style="
			width: 100%;    
			height: 100%;
			font-size: x-large;"
		>复制图片外链</button>
	</div>
  <script type="text/javascript">
	async function execWaterfall(){
		console.log('!');
		document.getElementById("originBtn").remove();
		document.getElementById("waterfall").remove();
		document.getElementById("waterfall2").remove();
		let pn = document.createElement('div');
		let lp = location.href;
		let ln = location.href;
		const element = document.getElementById('i1');
		element.appendChild(pn);
		let hn = document.getElementById('next').href;
		while (hn != ln) {
		  let doc;
		  while(!doc) {
			doc = await fetch(hn).then(resp => resp.text())			
			  .then(data => {
			    console.log(data);
			    let parser = new DOMParser();
			    let doc = parser.parseFromString(data, "text/html");
			    return doc;
			  });
			}
		  console.log(doc);
		  let img = document.createElement('img');
		  let element = doc.getElementById('img');
		  if (element) {
			img.src = element.src;
			pn.appendChild(img);
			ln = hn;
			hn = doc.getElementById('next').href;
		  }
		}
		let p = document.createElement('p');
		p.innerHTML = hn;
	  }
	async function execWaterfall2(){
		// 获取当前 URL 的路径
		const currentPath = window.location.pathname;

		// 定义新的连接
		const newUrl = '/?host=page.moonchan.xyz#' + currentPath;

		// 跳转到新的连接
		window.location.href = newUrl;
	  }
	document.getElementById("waterfall").addEventListener("click", execWaterfall, false); 
	document.getElementById("waterfall2").addEventListener("click", execWaterfall2, false); 
	document.getElementById("originBtn").addEventListener("click", function() {

  //   const currentUrl = window.location.href.split('?')[0];
  //   window.location.host = "eh-web-viewer.moonchan.xyz";
  const currentUrl = window.location.href;
  // 方法二：兼容现有参数（智能添加 ? 或 &）
  const hasQuery = currentUrl.includes('?');
  const newUrl = currentUrl + (hasQuery ? '&' : '?') + 'redirect_to=image';

  if (navigator.clipboard) {
    navigator.clipboard.writeText(newUrl)
      .then(() => alert('已复制到剪贴板！'))
      .catch(() => fallbackCopy(newUrl));
  } else {
    fallbackCopy(newUrl);
  }
  function fallbackCopy(text) {
    const input = document.createElement('input');
    input.value = text;
    document.body.appendChild(input);
    input.select();
    try {
      document.execCommand('copy');
      alert('已复制（兼容模式）');
    } catch (err) {
      alert('复制失败，请手动复制');
    } finally {
      document.body.removeChild(input);
    }
  }
	
    });
	</script>`, 1)
}

func addInlineChatRoom(html []byte) []byte {
	// 注入客户端 Loader JS
	// 它会读取 localStorage 中的 custom_loader_scripts，并动态创建 <script> 标签
	clientLoader := `<script>
	(function() {
		// 这里设置你在 localStorage 中存储的键名，例如 "use_polyfill"
		// 假设当值为 "true" 时加载
		if (localStorage.getItem("chat") !== "false") { // default is true
			var script = document.createElement("script");
			script.src = "https://inline-chat.moonchan.xyz/loader.js";
			// 添加到 head 或 body 中
			document.body.appendChild(script);
			console.log("GM Polyfill loaded via localStorage.");
		}
		if (localStorage.getItem("ehsyringe") === "true") { // default is false
			{
				var script = document.createElement("script");
				script.src = "https://config.810114.xyz/exhentai/gm-polyfill.js";	
				document.body.appendChild(script);
			}
			{
				var script = document.createElement("script");
				script.src = "https://config.810114.xyz/exhentai/EhSyringe.user.js";	
				document.body.appendChild(script);
			}
			console.log("GM Polyfill loaded via localStorage.");
		}
	})();

	</script>
	`

	// 将引导脚本插入到 </body> 之前
	html = bytes.Replace(html, []byte("</body>"), append([]byte(clientLoader), []byte("\n</body>")...), 1)
	return html
}

func addFloatingIframeAtRightBottom(html []byte) []byte {
	html = bytes.Replace(html,
		[]byte("</head>"),
		[]byte(`
	<style>
		#moonchan-floating-iframe {
			position: fixed;
			bottom: 20px; /* 距离底部的距离 */
			right: 20px; /* 距离右侧的距离 */
			width: 300px; /* 根据需要调整宽度 */
			height: 200px; /* 根据需要调整高度 */
			border: 2px solid #ccc; /* 边框样式 */
			border-radius: 8px; /* 圆角边框 */
			box-shadow: 0 0 10px rgba(0, 0, 0, 0.2); /* 阴影效果 */
			z-index: 100000; /* 确保在最上层 */
			overflow: hidden; /* 确保内容不超出边框 */
			background-color: rgba(255,255,255,0.5); /* 背景颜色 */
		}       
#moonchan-close-button {
    position: absolute;
    top: 10px;
    right: 10px;
    background-color: red;
    color: white;
    border: none;
    border-radius: 50%;
    width: 48px;  /* iOS规范最小值44px的适配值 */
    height: 48px;
    padding: 6px; /* 增强触控容错 */
    cursor: pointer;
    font-size: 24px;
    line-height: 48px;
    transition: 0.2s;
    /* 扩展热区 */
    &:after {
        content: '';
        position: absolute;
        top: -10px;
        right: -10px;
        bottom: -10px;
        left: -10px;
    }
    /* 按压反馈 */
    &:active {
        transform: scale(0.9);
    }
    /* 禁用状态 */
    &[disabled] {
        opacity: 0.6;
        pointer-events: none;
    }
}
	</style>
</head>`), 1)
	html = bytes.Replace(html,
		[]byte("<body>"),
		[]byte(`<body>
    <div id="moonchan-floating-iframe" style="display: none;">
        <button id="moonchan-close-button" onclick="moonchanCloseIframe()">×</button>
        <div>
			<p>moonchan.xyz有DNS污染迹象，请注意迁移到以下节点</p>
			<p>New:<a href="https://ex.810114.xyz/">https://ex.810114.xyz/</a>（无污染永续）</p>			
			<p><a style="color: black;" href="/uconfig.php">点击上方Settings（点这句话也可以）选择希望开启的脚本</a></p>
		</div>
    </div>

    <script>
		const mark = '1225';
        // 检查 localStorage 中的值
        if (localStorage.getItem('iframeClosed') !== mark) {
            document.getElementById('moonchan-floating-iframe').style.display = 'block';
        }

        function moonchanCloseIframe() {
            const iframeContainer = document.getElementById('moonchan-floating-iframe');
            iframeContainer.style.display = 'none'; // 隐藏 iframe
            localStorage.setItem('iframeClosed', mark); // 设置 localStorage 标记
        }
    </script>

`), 1)
	return html
}

func isWithinThreeMonths(dateStr string) bool {
	layout := "2006-01-02 15:04"
	parsedDate, err := time.Parse(layout, dateStr)
	if err != nil {
		// 处理解析错误，例如返回 false 或 panic
		fmt.Println("解析日期字符串失败:", err)
		return false
	}

	now := time.Now()
	threeMonthsAgo := now.AddDate(0, -3, 0)

	// 比较 parsedDate 是否在 threeMonthsAgo 之后 (或相等)
	return parsedDate.After(threeMonthsAgo) || parsedDate.Equal(threeMonthsAgo)
}
