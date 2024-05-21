#!/bin/bash

go build .

export GOOS=linux
export GOARCH=amd64
export CGO_ENABLED=0
# export CC=aarch64-linux-gnu-gcc

go build -o api-pack.bin .


# ./download.sh api-pack.bin api-pack-gin