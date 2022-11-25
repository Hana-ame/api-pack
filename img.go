package main

import (
	"fmt"
	"io"
	"net/http"
)

var old string

func img(w http.ResponseWriter, r *http.Request) {

	// kv_mu.RLock()
	// old = kv_obj["img_old"]
	// kv_mu.RUnlock()

	// path := strings.Replace(r.URL.String(), old, "/img/api/download/", 1)
	path := "/img/api/download/" + r.URL.String()[len("/xxxxxxxx/"):]

	fmt.Println(path)

	resp, err := http.Get("https://moonchan.xyz" + path)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	w.Header().Set(`Access-Control-Allow-Origin`, `*`)
	w.Header().Set(`Cache-Control`, `public, max-age=31536000`)

	io.Copy(w, resp.Body)

}
