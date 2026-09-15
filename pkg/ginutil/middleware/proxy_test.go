package middleware

import (
	"fmt"
	"net/url"
	"testing"
)

func TestUrl(t *testing.T) {
	href, _ := url.Parse("https://moonchan.xyz:443/path/to?q#aa")

	fmt.Println(href)
	fmt.Println(href.Host)
	fmt.Println(href.Scheme)
	fmt.Println(href.RawPath)
	fmt.Println(href.Path)
	fmt.Println(href.Query())

}
