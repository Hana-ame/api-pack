package main

import (
	"fmt"
	"net/url"
	"testing"
)

func TestUrlValues(t *testing.T) {
	v := make(url.Values)
	s := v.Encode()
	fmt.Println(s, len(v)) // ""
	v.Add("key", "value")
	s = v.Encode()
	fmt.Println(s, len(v)) // "key=value"
	v.Add("key2", "value2")
	s = v.Encode()
	fmt.Println(s, len(v)) // "key=value&key2=value2"
	v.Del("key2")
	s = v.Encode()
	fmt.Println(s, len(v)) // "key=value&key2=value2"
}
