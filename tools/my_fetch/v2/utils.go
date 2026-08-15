package myfetch

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

func ResponseToReader(r *http.Response) (reader io.Reader, err error) {
	switch r.Header.Get("Content-Encoding") {
	case "gzip":
		return gzip.NewReader(r.Body)
	case "br":
		r := brotli.NewReader(r.Body)
		if r == nil {
			err = fmt.Errorf("header is br and failed to make new reader")
		}
		return r, err
	case "zstd":
		return zstd.NewReader(r.Body)
	default:
		return r.Body, nil
	}
}
