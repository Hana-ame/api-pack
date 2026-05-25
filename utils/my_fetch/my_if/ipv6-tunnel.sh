#!/bin/bash

# ================= 变量配置 =================
HE_SERVER="66.220.18.42"         # HE 隧道服务器 IPv4
LOCAL_IPV4="142.171.157.74"      # 本机 IPv4
HE_CLIENT_IP="2001:470:c:6c::2"  # 你的 IPv6 地址 (不带 /64)
# ===========================================

echo "1. 清理旧隧道..."
ip tunnel del sit1 2>/dev/null
ip -6 route flush default 2>/dev/null

echo "2. 创建隧道..."
ip tunnel add sit1 mode sit remote $HE_SERVER local $LOCAL_IPV4 ttl 255
ip link set sit1 up

echo "3. 配置 IP 地址..."
# ip -6 addr add "$HE_CLIENT_IP/64" dev sit1

echo "4. 修正路由表..."
# 关键修正：
# 1. 再次确保删除所有默认路由
ip -6 route flush default

# 2. 添加 HE 为默认路由
# 不使用 'via' (点对点隧道不需要，或者应该填 ::1)，但必须保留 'src' 以锁定源 IP
ip -6 route add default dev sit1 src "$HE_CLIENT_IP" metric 1

# (可选) 再次尝试屏蔽 eth0 的 RA 干扰
sysctl -w net.ipv6.conf.eth0.accept_ra=0 >/dev/null

echo "配置完成。"
echo "--------------------------------"
echo "路由表检查 (现在应该有 default dev sit1):"
ip -6 route show default
echo "--------------------------------"
echo "连接测试:"
ping6 -c 3 google.com