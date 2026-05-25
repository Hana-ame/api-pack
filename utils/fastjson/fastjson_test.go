package fastjson

import (
	"fmt"
	"log"
	"testing"

	"github.com/valyala/fastjson"
)

func TestGet(t *testing.T) {
	s := []byte(`{"foo": [123, "bar"]}`)
	fmt.Printf("foo.0=%d\n", fastjson.GetInt(s, "foo", "0"))

	// Output:
	// foo.0=123
}

func TestPhaser(t *testing.T) {
	var p fastjson.Parser
	v, err := p.Parse(`{
                "str": "bar",
                "int": 123,
                "float": 1.23,
                "bool": true,
                "arr": [1, "foo", {}]
        }`)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("foo=%s\n", v.GetStringBytes("str"))
	fmt.Printf("int=%d\n", v.GetInt("int"))
	fmt.Printf("float=%f\n", v.GetFloat64("float"))
	fmt.Printf("bool=%v\n", v.GetBool("bool"))
	fmt.Printf("arr.1=%s\n", v.GetStringBytes("arr", "1"))

	// Output:
	// foo=bar
	// int=123
	// float=1.230000
	// bool=true
	// arr.1=foo
}
