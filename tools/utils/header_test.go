package tools

import (
	"log"
	"net/http"
	"testing"
)

func TestHeader(t *testing.T) {
	header := Header{http.Header{}}
	header.Add("Accept", "11")
	header.Add("Accept", "")
	header.Add("Accept", "1232")
	header.Add("Accept2", "")

	log.Println(header)
}
