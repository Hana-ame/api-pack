package main

import (
	"api-pack/helper"
	"api-pack/img"
	"api-pack/kv"
	"api-pack/nodeinfo"
	"api-pack/reflect"
	"api-pack/sign"

	"net/http"

	"github.com/gorilla/mux"
)

func main() {
	r := mux.NewRouter()
	// auto generate
	r.HandleFunc("/helper/", helper.HandleRoot)
	r.HandleFunc("/helper/{peer}", helper.HandlePeer)
	r.HandleFunc("/43df14f5/", img.Img)
	r.HandleFunc("/img/", img.Img)
	r.HandleFunc("/c124a90c/", kv.KV)
	r.HandleFunc("/kv/", kv.KV)
	r.HandleFunc("/nodeinfo/", nodeinfo.Nodeinfo)
	r.HandleFunc("/reflect/", reflect.Reflect)
	r.HandleFunc("/8b92d4de/", sign.Sign)
	r.HandleFunc("/sign/", sign.Sign)

	// end auto generate
	http.ListenAndServe("127.111.111.111:8080", r)
}
