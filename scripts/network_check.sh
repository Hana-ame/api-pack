#!/bin/bash

echo "=== Cloudflare DNS 查询 ==="
echo "Cloudflare DNS (1.1.1.1):"
nslookup cloudflare.com 1.1.1.1 2>/dev/null || echo "nslookup failed, trying dig..."
dig @1.1.1.1 cloudflare.com +short 2>/dev/null || echo "dig not available"

echo ""
echo "Cloudflare DNS (1.0.0.1):"
nslookup cloudflare.com 1.0.0.1 2>/dev/null || echo "nslookup failed"

echo ""
echo "=== Cloudflare API 测试 ==="
curl -s -o /dev/null -w "HTTP Code: %{http_code}\nTime: %{time_total}s\n" https://cloudflare.com/cdn-cgi/trace 2>/dev/null || echo "curl failed"

echo ""
echo "=== CloudCone 网络测试 ==="
# CloudCone 主要是一个 VPS 提供商，这里测试其官网
echo "Testing cloudcone.net:"
curl -s -o /dev/null -w "HTTP Code: %{http_code}\nTime: %{time_total}s\n" https://cloudcone.net 2>/dev/null || echo "curl failed"

echo ""
echo "=== 本地网络接口信息 ==="
ifconfig 2>/dev/null || ip addr show 2>/dev/null || echo "Network info commands not available"

echo ""
echo "=== DNS 解析测试 ==="
echo "Resolving pbs.twimg.com:"
nslookup pbs.twimg.com 2>/dev/null || echo "nslookup failed"

echo ""
echo "Resolving video.twimg.com:"
nslookup video.twimg.com 2>/dev/null || echo "nslookup failed"
