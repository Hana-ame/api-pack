#!/bin/bash

export CGO_ENABLED=0

# 构建 Go 项目
go build -ldflags="-s -w" -o ex.bin

# 检查构建是否成功
if [ $? -ne 0 ]; then
    echo "构建失败，退出。"
    exit 1
fi

# ~/script/scp.sh -v -P26275 ex.bin root@bwh.moonchan.xyz:~/temp
# 检查 SCP 是否成功
if [ $? -ne 0 ]; then
    echo "文件传输失败，退出。"
    exit 1
fi  

~/script/scp.sh -v -r exhentai root@cloudcone.moonchan.xyz:~/exhentai_modify/
# ~/script/scp.sh -v sw.js root@bwh.moonchan.xyz:~/exhentai/
# ~/script/scp.sh -v -P26275 -r exhentai root@bwh.moonchan.xyz:~
# ~/script/scp.sh -v failed.html root@bwh.moonchan.xyz:~/exhentai/
date;
echo "done"
# exit 0