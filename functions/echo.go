package functions

import (
	"api-pack/Tools/orderedmap"
	"fmt"
	"io"
	"log"
	"sort"

	"github.com/gin-gonic/gin"
)

func Echo(c *gin.Context) {

	println := func(format string, a ...any) {
		str := fmt.Sprintf(format, a...)
		c.String(200, (str)+"\n")
	}

	println(`----------head----------`)
	println(c.Request.Method)
	println(c.Request.Host)
	println("%v", c.Request.URL)
	println(c.Request.Proto)

	o := orderedmap.New()
	for k, v := range c.Request.Header {
		o.Set(k, v)
	}
	o.SortKeys(sort.Strings)

	for _, k := range o.Keys() {
		for _, v := range o.GetOrDefault(k, []string{"!error!"}).([]string) {
			println("%v: %v", k, v)
		}
	}
	println(`----------body----------`)

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Fatal(err)
		println("%v", err)
	} else {
		println(string(body))
	}
	println(`----------end of body----------`)

}
