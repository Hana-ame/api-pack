package main

import (
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"

	tools "github.com/Hana-ame/api-pack/Tools"
	"github.com/Hana-ame/api-pack/Tools/debug"
	myfetch "github.com/Hana-ame/api-pack/Tools/my_fetch"
	"github.com/Hana-ame/api-pack/Tools/my_fetch/my_if"
	middleware "github.com/Hana-ame/api-pack/Tools/my_gin_middleware"
	"github.com/gin-gonic/gin"
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

func ExhProxy() {
	debug.LogLevel = debug.Fatal

	prefix := "2001:470:c:6c:"

	ips := []net.IP{my_if.NewAddr(prefix), my_if.NewAddr(prefix)}
	ipidx := 0

	my_if.AddAddr(ips[ipidx].String())
	cp := myfetch.NewClientPool([]*http.Client{
		myfetch.NewV6Client(ips[ipidx]),
	})

	mf := myfetch.NewFetcher(nil, cp)

	// gin
	r := gin.Default()

	// 设置 CORS 头
	r.Use(middleware.CORSMiddleware())

	// 定义一个简单的 GET 路由
	r.Any("/*any", func(c *gin.Context) {
		c.Header("X-Debug-Request-Host", c.Request.Host)     // 要设置 Host $http_host
		c.Header("X-Debug-Header-Host", c.GetHeader("Host")) // never

		if c.Request.Body != nil {
			defer c.Request.Body.Close()
		}
		// 读取 URL 参数
		path := c.Request.URL.String()

		if strings.HasPrefix(path, "/api") {
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}

		if strings.HasPrefix(path, "/fullimage") {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		host := "exhentai.org"

		header := tools.NewHeader(c.Request.Header)
		header.Set("Cookie", "ipb_member_id=5698562; ipb_pass_hash=154e574fd19294c32f905fe187cbdad1; yay=louder; igneous=5eevdxac75hpx71cv")

		resp, err := mf.Fetch(
			c.Request.Method, "https://"+host+path,
			(header.Header), c.Request.Body)
		if err != nil {
			debug.E("why", err.Error())
			c.Header("X-Error", err.Error())
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}
		defer resp.Body.Close()

		if mf.Count() > 1000 {
			ipidx = (ipidx + 1) % len(ips)
			my_if.DelAddr(ips[ipidx].String())
			ips[ipidx] = my_if.NewAddr(prefix)
			my_if.AddAddr(ips[ipidx].String())
			newCp := myfetch.NewClientPool([]*http.Client{myfetch.NewV6Client(ips[ipidx])})
			mf.SetClientPool(newCp)
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

		// data = bytes.ReplaceAll(data, []byte("https://exhentai.org/g/"), []byte("https://"+"ex.nmbyd1.top"+"/g/"))
		// data = bytes.ReplaceAll(data, []byte("https://exhentai.org/s/"), []byte("https://"+"ex.nmbyd1.top"+"/s/"))
		// data = bytes.ReplaceAll(data, []byte("https://exhentai.org/z/"), []byte("https://"+"ex.nmbyd1.top"+"/z/"))
		// data = bytes.ReplaceAll(data, []byte("https://exhentai.org/img/"), []byte("https://"+"ex.nmbyd1.top"+"/img/"))
		data = bytes.ReplaceAll(data, []byte("https://exhentai.org"), []byte("https://"+"ex.nmbyd1.top"))
		c.Header("X-Debug", c.GetHeader("Host"))
		data = bytes.ReplaceAll(data, []byte("https://s.exhentai.org"), []byte("https://s-ex.moonchan.xyz"))
		if strings.HasPrefix(path, `/s/`) {
			data = []byte(addWaterFallViewButton(string(data)))
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

		// 获得param，redirect_to=image
		// 没经过类型检查
		if c.Query("redirect_to") == "image" {
			if !strings.HasPrefix(path, "/s/") {
				c.AbortWithStatus(http.StatusServiceUnavailable)
				return
			}
			doc, err := htmlquery.Parse(bytes.NewReader(data))
			if err != nil {
				c.Header("X-Error", err.Error())
				c.AbortWithStatus(http.StatusBadGateway)
				return
			}

			image, err := findOneAndSelectAttr(doc, "//img[@id='img']", "src")
			c.Redirect(http.StatusFound, image)
			return
		}

		// log.Println(string(data[:1024]))
		if len(data) == 0 {
			debug.E("why", resp.Status)
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
	r.Run("127.25.23.2:8080") // 在 8080 端口启动服务

}

func SProxy() {
	r := gin.Default()

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
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}

		// if strings.HasPrefix(path, "/fullimg/") {
		// 	c.AbortWithStatus(http.StatusServiceUnavailable)
		// 	return
		// }

		host := "s.exhentai.org"

		header := tools.NewHeader(c.Request.Header)

		resp, err := myfetch.Fetch(
			c.Request.Method, "https://"+host+path,
			(header.Header), c.Request.Body)
		if err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
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
			"X-Host":    host,
			"X-Origin":  header.Get("Origin"),
			"X-Referer": header.Get("Referer"),
			"X-Cookie":  header.Get("Cookie"),
		})

	})

	// 启动服务器
	r.Run("127.25.23.3:8080") // 在 8080 端口启动服务

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
	</div>
  <script type="text/javascript">
	async function execWaterfall(){
		console.log('!');
		document.getElementById("waterfall").remove();
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
	document.getElementById("waterfall").addEventListener("click", execWaterfall, false); 
	</script>`, 1)
}
