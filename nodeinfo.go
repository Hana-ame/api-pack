package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"
)

const prefix = 3

type node struct {
	m map[string]int64
	sync.Mutex
}

var mu sync.RWMutex
var site map[string]*node

func deamon() {
	for {
		time.Sleep(time.Hour / 6)
		now := time.Now().UnixNano()
		mu.Lock()
		for k, v := range site {
			v.Lock()
			flag := true
			for _, time := range v.m {
				if now-time > 1800*1e9 {
					continue
				}
				flag = false
				break
			}
			if flag {
				delete(site, k)
			}
			v.Unlock()
		}
		mu.Unlock()
	}
}

func nodeinfo(w http.ResponseWriter, r *http.Request) {
	if site == nil {
		site = make(map[string]*node)
		go deamon()
	}
	arr := strings.Split(r.URL.String(), `/`)
	if len(arr) < prefix {
		return
	}
	key := arr[prefix-1]
	if len(arr) > prefix {
		cmd := arr[prefix]
		if cmd == `clear` {
			clear(key)
		}
	}
	if r.Method == `POST` {
		defer r.Body.Close()
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError),
				http.StatusInternalServerError)
			return
		}

		set(key, string(body))
	}
	w.Write(get(key))
}

func getnode(key string) *node {
	mu.RLock()
	n := site[key]
	if n == nil {
		mu.RUnlock()
		n = &node{
			m: make(map[string]int64),
		}
		mu.Lock()
		site[key] = n
		mu.Unlock()
		mu.RLock()
	}
	mu.RUnlock()
	return n
}

func clear(key string) {
	// log.Println(`clear`, key)
	mu.Lock()
	delete(site, key)
	mu.Unlock()
}
func set(key, data string) {
	// log.Println(`set`, key)
	n := getnode(key)
	n.Lock()
	n.m[data] = time.Now().UnixNano()
	n.Unlock()
}
func get(key string) []byte {
	// log.Println(`get`, key)
	n := getnode(key)
	n.Lock()
	r, err := json.Marshal(n.m)
	if err != nil {
		r = []byte(`{}`)
	}
	n.Unlock()
	return r
}
