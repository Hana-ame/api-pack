import hmac
from hashlib import sha1
import base64
import time
import uuid

def make_sign():
    """
    生成签名
    """

    # API访问密钥
    secret_key = 'WgdHoQ70PstycjjPkH-3smOk2OA1zTrK'

    # 请求API接口的uri地址
    uri = "/api/generate/webui/text2img/ultra"
    # 当前毫秒时间戳
    timestamp = str(int(time.time() * 1000))
    # 随机字符串
    signature_nonce= str(uuid.uuid4())
    # 拼接请求数据
    content = '&'.join((uri, timestamp, signature_nonce))
    
    # 生成签名
    digest = hmac.new(secret_key.encode(), content.encode(), sha1).digest()
    # 移除为了补全base64位数而填充的尾部等号
    sign = base64.urlsafe_b64encode(digest).rstrip(b'=').decode()
    return sign


print(make_sign())