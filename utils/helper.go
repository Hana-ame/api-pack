package helper

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/mux"
)

var root map[string]string
var peer map[string][]string

var mu sync.RWMutex
var client *http.Client

func Append(host string, name, laddr string) {
	_, err := http.Post(host+name, "application/json", strings.NewReader(laddr))
	if err != nil {
		log.Println(err)
	}
}

func Get(host string, name string) []string {
	r, err := http.Get(host + name)
	if err != nil {
		log.Println(err)
		return nil
	}
	if r.StatusCode != http.StatusOK {
		return nil
	}
	var arr []string
	err = json.NewDecoder(r.Body).Decode(&arr)
	if err != nil {
		log.Println(err)
		return nil
	}
	return arr
}

func Post(host string, name, laddr string) map[string]string {
	r, err := http.Post(host, "application/json", strings.NewReader(name+","+laddr))
	if err != nil {
		log.Println(err)
		return nil
	}
	defer r.Body.Close()
	var m map[string]string
	err = json.NewDecoder(r.Body).Decode(&m)
	if err != nil {
		log.Println(err)
		return nil
	}
	return m
}

func Delete(host string, name string) {
	if client == nil {
		client = &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	req, err := http.NewRequest("DELETE", host, strings.NewReader(name))
	if err != nil {
		log.Println(`req, err := http.NewRequest("DELETE", host, strings.NewReader(name))`)
		log.Println(err)
	}
	_, err = client.Do(req)
	if err != nil {
		log.Println("r, err := client.Do(req)")
		log.Println(err)
	}
}

func Server(host string) error {
	root = make(map[string]string)
	peer = make(map[string][]string)

	r := mux.NewRouter()
	r.HandleFunc("/", handleRoot)
	r.HandleFunc("/{peer}", handlePeer)
	return http.ListenAndServe(host, r)
}

func handlePeer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	p, ok := vars["peer"]
	if !ok {
		http.Error(w, `peer, ok := vars["peer"]`, http.StatusBadRequest)
	}
	mu.RLock()
	_, ok = root[p]
	mu.RUnlock()
	if !ok {
		http.Error(w, `_, ok = root[peer]`, http.StatusNotFound)
	}
	mu.Lock()

	if r.Method == "GET" {
		if arr, ok := peer[p]; ok {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(arr)
			delete(peer, p)
		} else {
			http.Error(w, "if arr, ok := peer[p]; ok {", http.StatusNotFound)
		}
	} else if r.Method == "POST" {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		r.Body.Close()

		if arr, ok := peer[p]; ok {
			peer[p] = append(arr, string(body))
		} else {
			peer[p] = []string{string(body)}
		}
	}

	mu.Unlock()
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		mu.Lock()
		data, err := json.Marshal(root)
		mu.Unlock()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	} else if r.Method == "POST" {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		defer r.Body.Close()
		args := strings.Split(string(body), ",")
		if len(args) != 2 {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}

		w.Header().Set("Content-Type", "application/json")
		mu.Lock()
		if !(args[0] == "" || args[1] == "") {
			root[args[0]] = args[1]
		}
		json.NewEncoder(w).Encode(root)
		mu.Unlock()
	} else if r.Method == "DELETE" {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		defer r.Body.Close()
		args := strings.Split(string(body), ",")
		arg := args[0]
		mu.Lock()
		delete(root, arg)
		delete(peer, arg)
		mu.Unlock()
	}
}
