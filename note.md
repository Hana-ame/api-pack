# api-pack
    http.HandleFunc("/43df14f5/", img)
    http.HandleFunc("/img/", img)
    http.HandleFunc("/c124a90c/", kv)
    http.HandleFunc("/kv/", kv)
    http.HandleFunc("/d9e1b17a/", nodeinfo)
    http.HandleFunc("/nodeinfo/", nodeinfo)
    http.HandleFunc("/acccfaca/", reflect)
    http.HandleFunc("/reflect/", reflect)
    http.HandleFunc("/8b92d4de/", sign)
    http.HandleFunc("/sign/", sign)
## 更新方法

开启go live
在wsl下

```bash
python gen.py 
```

api-pack目录下

```bash
ps -ef | grep api-pack
kill 

# copy this to command line, target server
DIR=/path/to/dir
DIR=/var/www/moonchan # *_secret_no_comment_*
cd $DIR/api-pack/   
pkill api-pack
sleep 1
curl localhost:5500/api-pack > api-pack
nohup ./api-pack > /dev/null 2>&1 &
```

## kv.go

GET
POST
DELETE
没什么好写的

## infonode.go

原来的helper server。
peers.


## reflect.go

返回http的头


## 杂物

```go
package main

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
)

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/test/{para}", test)
	http.ListenAndServe("127.111.111.111:8080", r)
}

func test(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fmt.Println(vars)
	fmt.Println(r.URL.Path)
}
```
