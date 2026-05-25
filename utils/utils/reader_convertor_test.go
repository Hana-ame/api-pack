package tools

import (
	"io"
	"log"
	"strings"
	"testing"
)

func Test1111(t *testing.T) {
	r := NewCanResetOnceReader(strings.NewReader("test content"))
	b, e := io.ReadAll(r)
	log.Println(string(b), e)
	r.Seek(0, 0)
	b, e = io.ReadAll(r)
	log.Println(string(b), e)
	p, e := r.Seek(0, 1)
	log.Println(p, e)

	b, e = io.ReadAll(r)
	log.Println(string(b), e)

}
