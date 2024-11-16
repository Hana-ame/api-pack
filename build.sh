#!/bin/bash

# 设置环境变量
export CGO_ENABLED=0
export GOOS=linux
export GOARCH=amd64

# 构建 Go 项目
go build -o api-pack-new

# 检查构建是否成功
if [ $? -ne 0 ]; then
    echo "构建失败，退出。"
    exit 1
fi

# 使用 SCP 传输文件到远程服务器
~/script/scp.sh api-pack-new root@vps.moonchan.xyz:~/

# 检查 SCP 是否成功
if [ $? -ne 0 ]; then
    echo "文件传输失败，退出。"
    exit 1
fi

echo "done"