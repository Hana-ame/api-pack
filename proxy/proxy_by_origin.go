package proxy

import (
	"io"
	"net/http"
	"strings"
)

func Img(w http.ResponseWriter, r *http.Request) {
	arr := strings.Split(r.URL.String(), "/")
	if len(arr) < 2 {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Wrong URL format"))
		return
	}
	fn := arr[len(arr)-1]
	id := arr[len(arr)-2]
	path := dstBase + "/" + id + "/" + fn

	resp, err := http.Get(path)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	w.Header().Set(`Access-Control-Allow-Origin`, `*`)
	w.Header().Set(`Cache-Control`, `public, max-age=31536000`)

	io.Copy(w, resp.Body)
}
