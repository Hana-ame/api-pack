#!/bin/bash

# 设置环境变量
export CGO_ENABLED=0
export GOOS=linux
export GOARCH=amd64

# 构建 Go 项目
go build -ldflags="-s -w" -o api-pack-new

# 检查构建是否成功
if [ $? -ne 0 ]; then
    echo "构建失败，退出。"
    exit 1
fi

# 已经不需要了
# ~/script/scp.sh iframe.html root@vps.moonchan.xyz:/var/www/moonchan/iframe.html 

# 使用 SCP 传输文件到远程服务器
# 发送到bwh
# ~/script/scp.sh -v -P26275 api-pack-new root@bwh.moonchan.xyz:~/temp
# # 检查 SCP 是否成功
# if [ $? -ne 0 ]; then
#     echo "文件传输失败，退出。"
#     exit 1
# fi

# ls temp && { pkill api-pack-new; rm -f api-pack-new; mv temp api-pack-new; ls -l api-pack-new; nohup ./api-pack-new & } 

# 发送到vps
# ~/script/scp.sh -4 api-pack-new root@vps.moonchan.xyz:~/temp
# # 检查 SCP 是否成功
# if [ $? -ne 0 ]; then
#     echo "文件传输失败，退出。"
#     exit 1
# fi

~/script/scp.sh -4 api-pack-new root@cloudcone.moonchan.xyz:~/temp
# 检查 SCP 是否成功
if [ $? -ne 0 ]; then
    echo "文件传输失败，退出。"
    exit 1
fi

# ~/script/vps/ssh.sh "pkill api-pack-new; rm -f api-pack-new; mv temp api-pack-new; nohup ./api-pack-new &"

echo "done"
exit 0

#  ifconfig sit1 inet6 del  2001:470:c:6c:babe:c98e:1161:e26b/64
#  ifconfig sit1 inet6 del  2001:470:c:6c:c6bf:4514:1161:e26a/64
#  ifconfig sit1 inet6 del  2001:470:c:6c:2f15:169f:1161:e269/64

# 无缝运行
cd; ls temp && { pkill api-pack-new; rm -f api-pack-new; mv temp api-pack-new; ls -l api-pack-new; nohup ./api-pack-new & } 
cd; ls bak && { pkill api-pack-new; rm -f api-pack-new; mv bak api-pack-new; ls -l api-pack-new; nohup ./api-pack-new & } 

pkill api-pack-new; rm -f api-pack-new; mv 5.bin api-pack-new; chmod +x api-pack-new;  nohup ./api-pack-new & 
pkill api-pack-new; nohup ./api-pack-new & 

FILE="1.bin"; TARGET="api-pack-new"; URL="https://wsl-5500.moonchan.xyz/$FILE"; wget "$URL"; ls "$FILE" && { pkill "$TARGET"; rm -f "$TARGET"; mv "$FILE" "$TARGET"; chmod +x "$TARGET"; nohup "./$TARGET" & }

cd; ls temp && { pkill shijima.bin; rm -f shijima.bin; mv temp shijima.bin; ls -l shijima.bin; nohup ./shijima.bin & } 
