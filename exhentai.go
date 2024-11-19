package main

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"strings"

	tools "github.com/Hana-ame/api-pack/Tools"
	myfetch "github.com/Hana-ame/api-pack/Tools/my_fetch"
	middleware "github.com/Hana-ame/api-pack/Tools/my_gin_middleware"
	"github.com/gin-gonic/gin"
)

func ExhProxy() {
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

		host := "exhentai.org"

		header := tools.NewHeader(c.Request.Header)
		header.Set("Cookie", "ipb_member_id=5698562; ipb_pass_hash=154e574fd19294c32f905fe187cbdad1; yay=louder; igneous=5eevdxac75hpx71cv")

		resp, err := myfetch.Fetch(
			c.Request.Method, "https://"+host+path,
			(header.Header), c.Request.Body)
		if err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}
		defer resp.Body.Close()

		// resp 获得 text
		// 需要override https://exhentai.org/s/, https://exhentai.org/g/
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

		// data = bytes.ReplaceAll(data, []byte("https://exhentai.org/g/"), []byte("https://"+"ex.nmbyd1.top"+"/g/"))
		// data = bytes.ReplaceAll(data, []byte("https://exhentai.org/s/"), []byte("https://"+"ex.nmbyd1.top"+"/s/"))
		// data = bytes.ReplaceAll(data, []byte("https://exhentai.org/z/"), []byte("https://"+"ex.nmbyd1.top"+"/z/"))
		// data = bytes.ReplaceAll(data, []byte("https://exhentai.org/img/"), []byte("https://"+"ex.nmbyd1.top"+"/img/"))
		data = bytes.ReplaceAll(data, []byte("https://exhentai.org"), []byte("https://"+"ex.nmbyd1.top"))
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

		// log.Println(string(data[:1024]))
		if len(data) == 0 {
			c.AbortWithStatus(http.StatusBadGateway)
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

		// 为什么会报多写。
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
