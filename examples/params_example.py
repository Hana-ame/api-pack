#!/usr/bin/env python3
"""
URL-safe Base64 编码示例
用于生成 params 查询参数
"""

import base64
import json


def encode_params(obj: dict) -> str:
    """
    将字典转换为 URL-safe base64 编码字符串（无填充）
    
    Args:
        obj: 要编码的字典
        
    Returns:
        URL-safe base64 编码的字符串
    """
    json_str = json.dumps(obj)
    # 使用 urlsafe_b64encode 并移除填充字符 '='
    return base64.urlsafe_b64encode(json_str.encode()).rstrip(b'=').decode()


def decode_params(encoded: str) -> dict:
    """
    从 URL-safe base64 字符串解码为字典
    
    Args:
        encoded: URL-safe base64 编码的字符串
        
    Returns:
        解码后的字典
    """
    # 添加填充
    padding = 4 - len(encoded) % 4
    if padding != 4:
        encoded += '=' * padding
    
    json_str = base64.urlsafe_b64decode(encoded).decode()
    return json.loads(json_str)


if __name__ == "__main__":
    # 示例 1: 基本参数
    params1 = {
        "host": "example.com",
        "scheme": "https"
    }
    encoded1 = encode_params(params1)
    print(f"示例 1: {encoded1}")
    # 输出: eyJob3N0IjoiZXhhbXBsZS5jb20iLCJzY2hlbWUiOiJodHRwcyJ9
    
    # 示例 2: 完整参数
    params2 = {
        "host": "api.example.com",
        "referer": "https://myapp.com",
        "scheme": "https",
        "origin": "https://myapp.com"
    }
    encoded2 = encode_params(params2)
    print(f"示例 2: {encoded2}")
    
    # 构建完整 URL
    base_url = "http://localhost:8080/test/path"
    full_url = f"{base_url}?params={encoded2}&other=value"
    print(f"完整 URL: {full_url}")
    
    # 验证解码
    decoded = decode_params(encoded2)
    print(f"解码结果: {decoded}")
    
    # 可以使用 requests 库测试
    # import requests
    # response = requests.get(full_url)
    # print(response.text)
