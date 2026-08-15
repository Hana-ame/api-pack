// timeline-deamon @ 2023-12-26
// azure-go @ 2023-12-21

package myfetch

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/Hana-ame/api-pack/tools/orderedmap"
	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

// this function receive json request.
func ResponseToObject(r *http.Response) (o *orderedmap.OrderedMap, err error) {
	o = orderedmap.New()
	// b, _ := io.ReadAll(r.Body)
	// err = json.NewDecoder(bytes.NewReader(b)).Decode(&o)
	// if err != nil {
	// 	os.WriteFile("o.html", b, 0644) // 0644 是权限模式
	// }
	json.NewDecoder(r.Body).Decode(&o)
	return o, err
}

func ResponseToObjectArray(r *http.Response) (arr []*orderedmap.OrderedMap, err error) {
	err = json.NewDecoder(r.Body).Decode(&arr)
	return arr, err
}

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

func URLToJSON(url string) (o *orderedmap.OrderedMap, err error) {
	resp, err := Fetch(http.MethodGet, url, nil, nil)
	if err != nil {
		return
	}
	return ResponseToObject(resp)
}
func URLToJSONArray(url string) (arr []*orderedmap.OrderedMap, err error) {
	resp, err := Fetch(http.MethodGet, url, nil, nil)
	if err != nil {
		return
	}
	return ResponseToObjectArray(resp)
}
