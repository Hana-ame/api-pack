package main

import (
    "net/http"
)

func main() {
    // auto generation start
    http.HandleFunc("/acccfaca/", reflect)
    http.HandleFunc("/8b92d4de/", sign)
    // end of auto generation
    
    http.ListenAndServe("127.111.111.111:8080", nil)
}

