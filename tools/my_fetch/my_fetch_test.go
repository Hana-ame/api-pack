// gin-pack @ 2024-04-06

package myfetch

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"testing"
)

func TestFetch(t *testing.T) {
	fmt.Println("1")
	log.Println(os.Getenv("HTTPS_PROXY")) // it works
	resp, _ := Fetch("GET", "https://mstdn.jp/users/nanakananoka", http.Header{"Accept": []string{"application/activity+json"}, "User-Agent": []string{"myfetch/1.0.0"}}, nil)
	log.Println(resp.Header)
	log.Println(resp.Header.Get("Content-Type"))
	defer resp.Body.Close()
	s, _ := io.ReadAll(resp.Body)
	log.Println(string(s))
}
