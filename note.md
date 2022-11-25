# api-pack

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

# copy this to command line
DIR=/path/to/dir
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
