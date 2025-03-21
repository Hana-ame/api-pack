// 在 .env 中设置 EXHENTAI_PROXY_COOKIE 项目以更新Cookie
// 默认 "ipb_member_id=5698562; ipb_pass_hash=154e574fd19294c32f905fe187cbdad1; yay=louder; igneous=5eevdxac75hpx71cv"

package main

import (
	"bytes"
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

	tools "github.com/Hana-ame/api-pack/Tools"
	"github.com/Hana-ame/api-pack/Tools/debug"
	myfetch "github.com/Hana-ame/api-pack/Tools/my_fetch"
	"github.com/Hana-ame/api-pack/Tools/my_fetch/my_if"
	middleware "github.com/Hana-ame/api-pack/Tools/my_gin_middleware"
	streams "github.com/Hana-ame/api-pack/Tools/my_streams"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"golang.org/x/net/html"

	"github.com/antchfx/htmlquery"
)

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

// 妹有v6,毙了
func EhProxy() {
	godotenv.Load(".env")

	// debug.LogLevel = debug.Fatal

	// prefix := tools.NewSlice(
	// 	os.Getenv("EXHENTAI_PROXY_PREFIX"),
	// 	"2001:470:c:6c:",
	// ).FirstUnequal("")

	// ips := []net.IP{my_if.NewAddr(prefix), my_if.NewAddr(prefix)}
	// ipidx := 0

	// my_if.AddAddr(ips[ipidx].String())
	// cp := myfetch.NewClientPool([]*http.Client{
	// 	myfetch.NewV6Client(ips[ipidx], jar),
	// })

	// cp = nil // debug
	// mf := myfetch.NewFetcher(nil, cp)

	// gin
	r := gin.Default()

	// 设置block
	// r.Use(middleware.BlockMiddleware()) // 改到下面

	// 设置 CORS 头
	r.Use(middleware.CORSMiddleware())

	// 定义一个简单的 GET 路由
	r.Any("/*any", func(c *gin.Context) {
		// 禁止访问存档
		// if strings.HasPrefix(c.Request.URL.String(), "/archiver.php") {
		// 	c.AbortWithStatus(http.StatusForbidden)
		// 	return
		// }

	}, func(c *gin.Context) {
		// 禁止非大陆访问

		// 获取请求中的 Cookie
		cookie, err := c.Cookie("pass")

		// 如果 Cookie 包含 pass=pass，直接继续处理请求
		if err == nil && cookie == "pass" {
			return
		}

		if c.Query("redirect_to") != "" {
			return
		}

		// 如果不在 "CN", "" 中的任意一个。
		if !slices.Contains([]string{"CN"}, c.Request.Header.Get("Cf-Ipcountry")) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"message":      "禁止访问",
				"Cf-Ipcountry": c.Request.Header.Get("Cf-Ipcountry"),
			})
			return
		}

	}, func(c *gin.Context) {
		c.Header("X-Debug-Request-Host", c.Request.Host)     // 要设置 Host $http_host
		c.Header("X-Debug-Header-Host", c.GetHeader("Host")) // never

		if c.Request.Body != nil {
			defer c.Request.Body.Close()
		}
		// 读取 URL 参数
		path := c.Request.URL.String()

		host := "exhentai.org"

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
			c.Request.Method, "https://"+host+path,
			(header.Header), c.Request.Body)
		if err != nil {
			debug.E("why", err.Error())
			c.Header("X-Error", err.Error())
			c.AbortWithError(http.StatusBadGateway, err)
			return
		}
		defer resp.Body.Close()

		// if mf.Count() > 250 {
		// 	ipidx = (ipidx + 1) % len(ips)
		// 	defer func(ip string) {
		// 		time.Sleep(60 * time.Second)
		// 		my_if.DelAddr(ip)
		// 	}(ips[ipidx].String())
		// 	ips[ipidx] = my_if.NewAddr(prefix)
		// 	my_if.AddAddr(ips[ipidx].String())
		// 	newCp := myfetch.NewClientPool([]*http.Client{myfetch.NewV6Client(ips[ipidx], jar)})
		// 	mf.SetClientPool(newCp)
		// }

		if strings.HasPrefix(path, "/torrent") {
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
			debug.E("why", err.Error())
			c.Header("X-Error", err.Error())
			c.AbortWithError(http.StatusBadGateway, err)
			return
		}
		data = bytes.ReplaceAll(data, []byte("https://exhentai.org"), []byte{})

		if strings.HasPrefix(path, `/s/`) {
			data = []byte(addWaterFallViewButton(string(data)))
		} else if strings.HasPrefix(path, "/g/") {
			// nothing here
		} else {
			data = addReloadCoverButton(data)
		}
		data = addFloatingIframeAtRightBottom(data)

		// 为什么自带的方法这么贵物
		c.Writer.Header().Set("Content-Encoding", "identity")
		// 和最后的方法到底用哪个。
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
			debug.E("why", resp.Status) // 304
			c.Header("X-Error", resp.Status)
			c.AbortWithStatus(resp.StatusCode)
			return
		}

		// 为什么会报多写。
		c.DataFromReader(resp.StatusCode, int64(len(data)), resp.Header.Get("Content-Type"), bytes.NewReader(data), map[string]string{
			"X-Host":    host,
			"X-Origin":  header.Get("Origin"),
			"X-Referer": header.Get("Referer"),
			"X-Cookie":  header.Get("Cookie"),
		})

	})

	// 启动服务器
	r.Run("127.25.23.6:8080") // 在 8080 端口启动服务

}

func ExhProxy() { // 就是这个
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
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.ProxyMiddleware())

	// 定义一个简单的 GET 路由
	r.Any("/*any", func(c *gin.Context) {
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
			// 如果 Cookie 包含 pass=pass，直接继续处理请求
			if cookie, err := c.Cookie("pass"); err != nil || cookie == "pass" {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}

		}

		if strings.HasPrefix(c.Request.URL.String(), "/static/") {
			c.Redirect(http.StatusFound, "https://moonchan.xyz"+c.Request.URL.String())
			c.Abort()
			return
		}

		// 遗留问题, image这个path重定向到用param的请求
		if strings.HasPrefix(c.Request.URL.String(), "/image") {
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
			// 如果 Cookie 包含 pass=pass，直接继续处理请求
			if cookie, err := c.Cookie("pass"); err != nil || cookie == "pass" {
				c.String(http.StatusOK, "哪个小天才整天改设置里的filter改得搜索搜不出东西的")
				c.Abort()
				return
			}

		}

		// 如果写在img标签的src里面，那么request的accept头会有
		// 这里其实没有parse。要写吗。如果不行回头再找。要找吗。
		// 不行，炸了，以后折腾。
		// indexImage := strings.Index(c.GetHeader("Accept"), "image")
		// indexHtml := strings.Index(c.GetHeader("Accept"), "html")
		// if c.Query("redirect_to") == "" && uint(indexHtml) <= uint(indexImage) {
		// 	path := c.Request.URL.String()
		// 	if strings.HasPrefix(path, "/s/") {
		// 		parsedURL, err := url.Parse(path)
		// 		if err != nil {
		// 			c.Header("X-Error", parsedURL.String())
		// 			c.AbortWithStatus(http.StatusBadRequest)
		// 			return
		// 		}
		// 		query := parsedURL.Query()
		// 		query.Set("redirect_to", "image")
		// 		parsedURL.RawQuery = query.Encode()

		// 		c.Redirect(http.StatusFound, parsedURL.String())
		// 		c.Abort()
		// 		return
		// 	} else if strings.HasPrefix(path, "/g/") {
		// 		parsedURL, err := url.Parse(path)
		// 		if err != nil {
		// 			c.Header("X-Error", parsedURL.String())
		// 			c.AbortWithStatus(http.StatusBadRequest)
		// 			return
		// 		}
		// 		query := parsedURL.Query()
		// 		query.Set("redirect_to", "cover")
		// 		parsedURL.RawQuery = query.Encode()

		// 		c.Redirect(http.StatusFound, parsedURL.String())
		// 		c.Abort()
		// 		return
		// 	} else {
		// 		redirectURL := "https://moonchan.xyz/favicon.ico"

		// 		c.Redirect(http.StatusFound, redirectURL)
		// 		c.Abort()
		// 		return
		// 	}
		// }

	}, func(c *gin.Context) {
		// 单纯封禁非中国

		// redirect_to image 不阻止.
		if c.Query("redirect_to") != "" {
			return
		}

		// 适配自家阅读器
		if tools.Match(url.Parse(c.Request.Referer())).Result().Host == "page.moonchan.xyz" {
			return
		}

		// 获取请求中的 Cookie
		cookie, err := c.Cookie("pass")

		// 如果 Cookie 包含 pass=pass，不阻止
		if err == nil && cookie == "pass" {
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
			c.String(http.StatusOK, "为了节约3$以及防止一个域名用太久被墙，现已经迁移到：https://ex.nmbyd2.top\nhttps://ex.nmbyd1.top 将在5月2日过期，在这之前你依然能够使用这个域名\n这条警告消息被设置为只会出现在首次访问的5秒内，以防有人看不见\n类似这样的迁移频率大致为10月一次，嫌麻烦可以给我打钱，有钱就能续域名也不用这么换了")
			c.Abort()
			return
		}

		// 没用啊, 这样google进来不也是ok了的
		// 只要带有Referer就不阻止. 是上面的扩展.
		// if c.Request.Referer() != "" {
		// 	return
		// }

	}, func(c *gin.Context) { // 看看有多少余额用的
		path := c.Request.URL.String()
		if strings.HasPrefix(path, "/exchange.php") {
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
			c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, nil)

			c.Abort()
			return
		}
	}, func(c *gin.Context) {
		c.Header("X-Debug-Request-Host", c.Request.Host)     // 要设置 Host $http_host
		c.Header("X-Debug-Header-Host", c.GetHeader("Host")) // never

		if c.Request.Body != nil {
			defer c.Request.Body.Close()
		}
		// 读取 URL 参数
		path := c.Request.URL.String()

		const host = "exhentai.org"

		header := tools.NewHeader(c.Request.Header)
		header.Set(
			"Cookie",
			tools.NewSlice(
				c.GetHeader("X-Cookie"),
				os.Getenv("EXHENTAI_PROXY_COOKIE"),
				"ipb_member_id=5698562; ipb_pass_hash=154e574fd19294c32f905fe187cbdad1; yay=louder; igneous=5eevdxac75hpx71cv",
			).FirstUnequal(""),
		)

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

		if mf.Count() > 250 {
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

		if strings.HasPrefix(path, "/torrent") {
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
			debug.E("why", err.Error())
			c.Header("X-Error", err.Error())
			c.AbortWithError(http.StatusBadGateway, err)
			return
		}

		if len(data) == 0 {
			c.AbortWithStatus(resp.StatusCode) // 304 not modified
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

		if strings.HasPrefix(path, "/s/") && c.Query("redirect_to") == "origin" {
			doc, err := htmlquery.Parse(bytes.NewReader(data))
			if err != nil {
				c.Header("X-Error", err.Error())
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}

			arr := findAll(doc, "//a", "href")
			// fmt.Println(arr) // it's ok

			gallery, err := streams.First(arr, func(s string) bool {
				return strings.HasPrefix(s, "/g/")
			})
			if err != nil {
				c.Header("X-Error", err.Error())
				debug.E("origin", err.Error())
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}

			fullimg, err := streams.First(arr, func(s string) bool {
				return strings.HasPrefix(s, "/fullimg")
			})
			if err != nil {
				c.Header("X-Error", err.Error())
				image, _ := findOneAndSelectAttr(doc, "//img[@id='img']", "src")
				c.Redirect(http.StatusFound, image)
				c.Abort()
				return
			}
			// debug.I("origin", gallery, fullimg)
			galleryURL, err := url.Parse("https://" + c.Request.Host + gallery)
			if err != nil {
				c.Header("X-Error", err.Error())
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			query := galleryURL.Query()
			query.Set("redirect_to", "json")
			galleryURL.RawQuery = query.Encode()

			o, err := myfetch.URLToJSON(galleryURL.String())
			if err != nil {
				c.Header("X-Error", err.Error())
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			// blob, e := json.Marshal(o)
			// fmt.Println(string(blob), e)

			// if isWithinThreeMonths(o.GetOrDefault("date", "2000-01-01").(string)) || isGalleryAvailable(o.GetOrDefault("date", "2000-01-01").(string)) {
			// 没啦
			if isGalleryAvailable(o.GetOrDefault("date", "2000-01-01").(string)) {
				debug.I("origin", o.GetOrDefault("date", "notfound"))
				resp, err := mf.Fetch(http.MethodGet, "https://"+host+fullimg,
					(header.Header), nil)
				if err != nil {
					// debug.I("origin", err.Error())
					c.Header("X-Error", err.Error())
					c.AbortWithStatus(http.StatusInternalServerError)
					return
				}
				defer resp.Body.Close()
				// l, _ := resp.Location()
				// debug.I("origin", l.String())
				mf.AddCount(5)
				if resp.StatusCode != http.StatusFound {
					// str, err := io.ReadAll(resp.Body)
					body, err := myfetch.ResponseToReader(resp)
					if err != nil {
						c.Header("X-Error", err.Error())
						c.AbortWithError(http.StatusInternalServerError, err)
						return
					}
					str, err := io.ReadAll(body)
					if err != nil {
						c.Header("X-Error", err.Error())
					}
					c.String(resp.StatusCode, string(str))
					return
				} else { // Found
					c.Redirect(http.StatusFound, resp.Header.Get("Location"))
					return
				}
			}

			c.AbortWithStatus(http.StatusForbidden)
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
		c.Writer.Header().Set("Content-Encoding", "identity")
		// 和最后的方法到底用哪个。
		c.Writer.Header().Set("Content-Length", strconv.Itoa(len(data)))
		for k, vs := range resp.Header {
			if c.Writer.Header().Get(k) != "" {
				continue
			}
			for _, v := range vs {
				c.Writer.Header().Add(k, v)
			}
		}

		if slices.Contains([]int{http.StatusNotFound}, resp.StatusCode) {
			c.Redirect(http.StatusTemporaryRedirect, "https://"+c.Request.Host)
			c.Abort()
			return
		}

		// 为什么会报多写。
		c.DataFromReader(resp.StatusCode, int64(len(data)), resp.Header.Get("Content-Type"), bytes.NewReader(data), map[string]string{
			"X-Host":               host,
			"X-Origin":             header.Get("Origin"),
			"X-Referer":            header.Get("Referer"),
			"X-Cookie":             header.Get("Cookie"),
			"X-Debug-Request-Host": c.Request.Host,
			"X-Debug-Header-Host":  c.GetHeader("Host"),
		})

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
				"ipb_member_id=5698562; ipb_pass_hash=154e574fd19294c32f905fe187cbdad1; yay=louder; igneous=5eevdxac75hpx71cv",
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
		tools.CopyHeader(c, resp.Header)

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
			下拉式
	  </button>
	  <button id="waterfall2" style="
			width: 100%;    
			height: 100%;
			font-size: x-large;
	  ">
			下拉式
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
		z-index: 99;
		display: none;"
	>
		<button id="originBtn" style="
			width: 100%;    
			height: 100%;
			font-size: x-large;"
		>原图</button>
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
		const newUrl = 'https://page.moonchan.xyz/#' + currentPath;

		// 跳转到新的连接
		window.location.href = newUrl;
	  }
	document.getElementById("waterfall").addEventListener("click", execWaterfall, false); 
	document.getElementById("waterfall2").addEventListener("click", execWaterfall2, false); 
	document.getElementById("originBtn").addEventListener("click", function() {
      const currentUrl = window.location.href.split('?')[0];
      window.location.href = currentUrl + '?redirect_to=origin';
    });
	</script>`, 1)
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
		}       
		#moonchan-close-button {
            position: absolute;
            top: 5px;
            right: 5px;
            background-color: red; /* 按钮颜色 */
            color: white; /* 字体颜色 */
            border: none;
            border-radius: 50%;
            width: 25px;
            height: 25px;
            cursor: pointer;
            font-size: 18px;
            line-height: 25px; /* 垂直居中 */
            text-align: center;
        }
	</style>
</head>`), 1)
	html = bytes.Replace(html,
		[]byte("<body>"),
		[]byte(`<body>
    <div id="moonchan-floating-iframe" style="display: none;">
        <button id="moonchan-close-button" onclick="moonchanCloseIframe()">×</button>
        <iframe src="https://moonchan.xyz/iframe.html?date=250308" style="border: none; width: 100%; height: calc(100% - 30px);"></iframe>
    </div>

    <script>
		const mark = '250306';
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

func isGalleryAvailable(datestr string) bool {
	const layout = "2006-01-02 15:04"

	// 解析输入时间（UTC时区）
	parsedDate, err := time.ParseInLocation(layout, datestr, time.UTC)
	if err != nil {
		fmt.Println("解析日期字符串失败:", err)
		return false
	}

	now := time.Now().UTC()

	// 计算12个月前的时间
	twelveMonthsAgo := now.AddDate(0, -12, 0)

	// 判断是否在12个月内
	within12Months := !parsedDate.Before(twelveMonthsAgo)

	// 判断当前是否非高峰时段
	notPeak := !isPeakHour(now)

	return within12Months && notPeak
}

// 判断是否是高峰时段（UTC时区）
// "Peak hours" for this purpose is (in UTC) between 14:00 and 20:00 UTC Monday-Saturday, and between 05:00 and 20:00 UTC on Sundays.
func isPeakHour(t time.Time) bool {
	t = t.UTC()
	hour := t.Hour()
	weekday := t.Weekday()

	if weekday != time.Sunday { // 周一到周六
		return hour >= 14 && hour < 20
	} else { // 周日
		return hour >= 5 && hour < 20
	}
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
