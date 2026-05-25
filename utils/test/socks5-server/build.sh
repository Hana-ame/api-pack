
#!/bin/bash

# 设置环境变量
export CGO_ENABLED=0
export GOOS=linux
export GOARCH=amd64

# 构建 Go 项目
go build -o socks5-server

# 检查构建是否成功
if [ $? -ne 0 ]; then
    echo "构建失败，退出。"
    exit 1
fi


# 使用 SCP 传输文件到远程服务器
# 发送到bwh
~/script/scp.sh -v -P26275 socks5-server root@bwh.moonchan.xyz:~/socks5-server
# 检查 SCP 是否成功
if [ $? -ne 0 ]; then
    echo "文件传输失败，退出。"
    exit 1
fi

rm socks5-server

~/script/bwh/ssh.sh

exit 0