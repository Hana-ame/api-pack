package middleware

import (
	"net/http"
	"net/url"

	myfetch "github.com/Hana-ame/api-pack/utils/my_fetch"
	tools "github.com/Hana-ame/api-pack/utils/utils"
	"github.com/gin-gonic/gin"
)

// 必须设置 X-Scheme 开启功能, 只支持 X-Host... 方式的设置
func ProxyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {

		scheme := c.GetHeader("X-Scheme")

		if scheme != "" {

			requestURL := c.Request.URL.String()

			host := c.GetHeader("X-Host")
			// origin := c.GetHeader("X-Origin")
			// referer := c.GetHeader("X-Referer")

			href, err := url.Parse(requestURL)
			if err != nil {
				c.Header("X-Error", err.Error())
				c.AbortWithError(http.StatusInternalServerError, err)
				return
			}
			href.Host = host
			href.Scheme = scheme

			hrefString := href.String()

			header := tools.NewHeader(c.Request.Header)

			header.Set("Host", host)
			header.Set("Origin", c.GetHeader("X-Origin"))
			header.Set("Referer", c.GetHeader("X-Referer"))
			header.Set("Cookie", tools.Or(c.GetHeader("X-Cookie"), header.Get("Cookie")))

			// 不是流式的。
			// body, err := c.Request.GetBody()
			// if tools.AbortWithError(c, http.StatusBadRequest, err) {
			// 	return
			// }
			resp, err := myfetch.Fetch(c.Request.Method, hrefString,
				(header.Header), c.Request.Body)
			if err != nil {
				c.Header("X-Error", err.Error())
				c.AbortWithError(http.StatusBadGateway, err)
				return
			}
			defer resp.Body.Close()

			// 为什么自带的方法这么贵物
			// for k, vs := range resp.Header {
			// 	if c.Writer.Header().Get(k) != "" { // 擦,好像是因为自己改了什么ContentType所以不好直接弄.但是还是保留了吧.
			// 		continue
			// 	}
			// 	for _, v := range vs {
			// 		c.Writer.Header().Add(k, v)
			// 	}
			// }
			tools.PatchHeader(c, resp.Header)
			// slices.Sort(exposeHeaders)
			// c.Writer.Header().Add("Access-Control-Expose-Headers", strings.Join(exposeHeaders, ", "))

			c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, map[string]string{
				"X-Scheme":  scheme,
				"X-Host":    host,
				"X-Origin":  header.Get("Origin"),
				"X-Referer": header.Get("Referer"),
				"X-Cookie":  header.Get("Cookie"),
				"X-Href":    hrefString,
			})

			c.Abort()

		} else {

			c.Next()
		}
	}
}
