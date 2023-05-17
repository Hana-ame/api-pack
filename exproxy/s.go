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

		req, err := http.NewRequest(r.Method, newUrl.String(), r.Body)
		if err != nil {
			fmt.Println(`Error On NewRequest`)
			return
		}
		for k, v := range r.Header {
			switch k {
			case "Cookie":
				w.Header().Add(k, COOKIE)
			case "Origin":
				w.Header().Add(k, "https://exhentai.org")
			case "Referer":
				w.Header().Add(k, "https://exhentai.org/")
			default:
				w.Header().Set(k, v[0])
			}
		}
		// log.Println(r.Header)
		// log.Println(w.Header())

		resp, err := client.Do(req)
		if err != nil {
			fmt.Println(`Error On Do Request`)
			return
		}
		defer resp.Body.Close()

		statusCode := resp.StatusCode
		for k, v := range resp.Header {
			switch k {
			case "Access-Control-Allow-Origin":
				w.Header().Add(k, "https://ex.moonchan.xyz")
			default:
				w.Header().Set(k, v[0])
			}
		}
		// log.Println(resp.Header)

		w.WriteHeader(statusCode)
		io.Copy(w, resp.Body)

	})

	server := &http.Server{Addr: listen, Handler: handler}

	err := server.ListenAndServe()
	log.Println(err)
}
