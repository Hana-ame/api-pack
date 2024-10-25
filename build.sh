#!/bin/bash

# 不能运行可能是编码问题，用UTF-8保存一次试试

go build .

export GOOS=linux
export GOARCH=amd64
export CGO_ENABLED=0
# export CC=aarch64-linux-gnu-gcc

go build -o gin-pack.bin .

# ./download.sh gin-pack.bin gin-pack
ssh root@vps.moonchan.xyz "./download.sh gin-pack.bin gin-pack"