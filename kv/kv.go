package kv

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

var kv_obj map[string]string
var kv_mu sync.RWMutex

func kv_deamon() {
	for {
		// time.Sleep(time.Second)
		time.Sleep(time.Minute * 10)
		data, err := json.Marshal(kv_obj)
		if err != nil {
			log.Println(err)
		}
		err = os.WriteFile("kv.json", data, 0644)
		if err != nil {
			log.Println(err)
		}
	}
}

func KV(w http.ResponseWriter, r *http.Request) {
	if kv_obj == nil {
		data, err := os.ReadFile("kv.json")
		if err != nil {
			w.Write([]byte(err.Error()))
			return
		}
		err = json.Unmarshal(data, &kv_obj)
		if err != nil {
			w.Write([]byte(err.Error()))
			return
		}
		go kv_deamon()
	}
	params := strings.Split(r.URL.String(), "/")
	key := params[len(params)-1]
	if r.Method == `GET` {
		kv_mu.RLock()
		w.Write([]byte(kv_obj[key]))
		kv_mu.RUnlock()
	} else if r.Method == `POST` {
		defer r.Body.Close()
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError),
				http.StatusInternalServerError)
			return
		}
		kv_mu.Lock()
		kv_obj[key] = string(body)
		kv_mu.Unlock()
	} else if r.Method == `DELETE` {
		kv_mu.Lock()
		delete(kv_obj, key)
		kv_mu.Unlock()
	}
}
