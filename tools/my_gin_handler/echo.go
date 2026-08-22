package handler

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"

	"github.com/Hana-ame/api-pack/tools/orderedmap"

	"github.com/gin-gonic/gin"
)

func EchoJSON(c *gin.Context) {
	c.Header("X-Debug-Request-Host", c.Request.Host)     // 要设置 Host $http_host
	c.Header("X-Debug-Header-Host", c.GetHeader("Host")) // never

	o := orderedmap.New()
	for k, v := range c.Request.Header {
		o.Set(k, v)
	}
	o.SortKeys(sort.Strings)

	c.JSONP(http.StatusOK, o)

}

func Echo(c *gin.Context) {
	c.Header("X-Debug-Request-Host", c.Request.Host)     // 要设置 Host $http_host
	c.Header("X-Debug-Header-Host", c.GetHeader("Host")) // never

	writeLine := func(format string, a ...any) {
		str := fmt.Sprintf(format, a...)
		c.String(200, (str)+"\n")
	}

	writeLine(`----------head----------`)
	writeLine("%s", c.Request.Method)
	writeLine("%s", c.Request.Host)
	writeLine("%v", c.Request.URL)
	writeLine("%s", c.Request.Proto)

	o := orderedmap.New()
	for k, v := range c.Request.Header {
		o.Set(k, v)
	}
	o.SortKeys(sort.Strings)

	for _, k := range o.Keys() {
		for _, v := range o.GetOrDefault(k, []string{"!error!"}).([]string) {
			writeLine("%v: %v", k, v)
		}
	}
	writeLine(`----------body----------`)

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Fatal(err)
		writeLine("%v", err)
	} else {
		writeLine("%s", string(body))
	}
	writeLine(`----------end of body----------`)

}

func EchoCFIP(c *gin.Context) {
	ip := c.GetHeader("CF-Connecting-IP")
	c.String(http.StatusOK, ip)
	c.AbortWithStatus(200)
}
