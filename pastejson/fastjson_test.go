package pastejson

import (
	"fmt"
	"testing"

	"github.com/valyala/fastjson"
)

func TestAr(t *testing.T) {
	body := []byte(`{
		"@metaData":{
			"tags":["1","22"],
			"previous":[123,456]
		}
	}`)
	var p fastjson.Parser
	v, _ := p.ParseBytes(body)
	tags := v.GetArray("@metaData", "tags")
	_ = tags
	fmt.Println(tags)
	for _, item := range tags {
		fmt.Printf("%v\n", item)
	}
	previous := v.GetArray("@metaData", "previous")
	fmt.Println(previous)
	for _, item := range previous {
		fmt.Printf("%v\n", item.GetInt64())
	}
}
