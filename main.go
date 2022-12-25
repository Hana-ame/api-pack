package main

import (
"api-pack/helper"
"api-pack/sign"

	"net/http"

	"github.com/gorilla/mux"
)

func main() {
	r := mux.NewRouter()
	// auto generate
r.HandleFunc("/helper/",helper.Handle123)
r.HandleFunc("/8b92d4de/",sign.Sign)
r.HandleFunc("/sign/",sign.Sign)

	// end auto generate
	http.ListenAndServe("127.111.111.111:8080", r)
}

