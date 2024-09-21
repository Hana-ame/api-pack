websocket反代传输用的，另一半在Tools/wsmux里面
websocket的客户端还没做，在Tools/

指令
```sh
go  test -timeout 300m -run ^TestXxx$ api-pack/files -v

go.exe test -timeout 30m -run ^TestResponse$ api-pack/Tools/ws_mux/examples -v
```
目前访问失败的时候会进入一个死循环。
```sh
curl 127.0.0.1:8080/api/..source.txt/1 -v
```
这个指令是好的
```sh
curl 127.0.0.1:8080/api/request.go/1 -v
```
但是会返回1313长度的message。

奇迹，修好了
把接受到len=0的package会创建新conn给砍掉就行了

我觉得我应该画点图。

擦，没有中断啊
而且速度好慢。