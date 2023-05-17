package exproxy

import (
	"fmt"
	"io"
	"log"
	"net/http"
)

func S(listen string) {
	handler := http.NewServeMux()
	handler.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		newUrl := r.URL
		newUrl.Host = "s.exhentai.org"
		newUrl.Scheme = "https"

		req, err := http.NewRequest("GET", newUrl.String(), nil)
		if err != nil {
			fmt.Println(`Error On NewRequest`)
			return
		}
		req.Header.Set("Cookie", COOKIE)

		resp, err := client.Do(req)
		if err != nil {
			fmt.Println(`Error On Do Request`)
			return
		}
		defer resp.Body.Close()

		statusCode := resp.StatusCode
		for k, v := range resp.Header {
			w.Header().Set(k, v[0])
		}

		w.WriteHeader(statusCode)
		io.Copy(w, resp.Body)

	})

	server := &http.Server{Addr: listen, Handler: handler}

	err := server.ListenAndServe()
	log.Println(err)
}
