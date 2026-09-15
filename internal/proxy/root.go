package proxy

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	tools "github.com/Hana-ame/api-pack/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ProxyParams holds base64-encoded JSON parameters for the proxy
type ProxyParams struct {
	Host    string `json:"host"`
	Referer string `json:"referer"`
	Scheme  string `json:"scheme"`
	Origin  string `json:"origin"`
}

// RootProxyHandler 通用 HTTP 反向代理：
// 通过 proxy_host（或 X-Host 头）指定目标，proxy_scheme/X-Scheme 指定协议；
// proxy_origin/referer/cookie（或对应 X-* 头）覆盖发往上游的对应请求头。
// 支持 params 查询参数，接受 base64 编码的 JSON: {"host":"...","referer":"...","scheme":"...","origin":"..."}
func RootProxyHandler(c *gin.Context) {
	path := c.Request.URL.Path

	// 解析 params 参数（URL-safe base64 编码的 JSON）
	var params ProxyParams
	if paramsStr := c.Query("params"); paramsStr != "" {
		// 使用 URL-safe base64 解码（支持 - 和 _ 字符，无需填充）
		decoded, err := base64.RawURLEncoding.DecodeString(paramsStr)
		if err != nil {
			// 尝试带填充的版本
			decoded, err = base64.URLEncoding.DecodeString(paramsStr)
		}
		if err == nil {
			if jsonErr := json.Unmarshal(decoded, &params); jsonErr == nil {
				// 成功解析 params，使用其中的值作为默认值
			}
		}
	}

	host := tools.Or(c.Query("proxy_host"), c.GetHeader("X-Host"), params.Host)

	// --- 参数校验 ---
	if host == "" {
		if path == "/favicon.ico" {
			c.Redirect(http.StatusFound, "https://moonchan.xyz/favicon.ico")
		} else if path == "/" || path == "" {
			// 显示代理生成器页面
			c.Header("Content-Type", "text/html; charset=utf-8")
			c.String(http.StatusOK, generateProxyGeneratorHTML())
		} else {
			c.Redirect(http.StatusFound, "https://moonchan.xyz/")
		}
		return
	}

	// 构造新的 URL
	scheme := tools.Or(c.Query("proxy_scheme"), c.GetHeader("X-Scheme"), params.Scheme, "https")
	clientScheme := tools.ClientScheme(c)
	search := c.Request.URL.Query()
	// 删除代理专用参数，避免传给后端
	search.Del("proxy_host")
	search.Del("proxy_origin")
	search.Del("proxy_referer")
	search.Del("proxy_cookie")
	search.Del("proxy_scheme")
	search.Del("params")

	targetURL := fmt.Sprintf("%s://%s%s", scheme, host, path)
	if len(search) > 0 {
		targetURL += "?" + search.Encode()
	}

	// --- 构造请求 ---
	// 必须使用 http.NewRequest 来手动控制
	req, err := http.NewRequest(c.Request.Method, targetURL, c.Request.Body)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// --- 处理请求头 (Request Headers) ---
	for k, vv := range c.Request.Header {
		// 跳过逐段传输头 (Hop-by-hop headers)、客户端可伪造的 IP/转发痕迹头、
		// 以及浏览器偏好头 Upgrade-Insecure-Requests（会诱导源站 301 跳 https）
		if tools.IsHopByHop(k) || tools.IsClientIPHeader(k) || tools.IsProxySpoofHeader(k) || strings.EqualFold(k, "Upgrade-Insecure-Requests") {
			continue
		}
		for _, v := range vv {
			req.Header.Add(k, v)
		}
	}

	// 关键：设置正确的 Host
	// 在 Go 中，req.Header.Set("Host", ...) 会被忽略，必须直接设置 req.Host
	req.Host = host

	// 覆盖特定的 Header
	req.Header.Set("Origin", tools.Or(c.Query("proxy_origin"), c.GetHeader("X-Origin"), params.Origin))
	req.Header.Set("Referer", tools.Or(c.Query("proxy_referer"), c.GetHeader("X-Referer"), params.Referer))
	if cookie := tools.Or(c.Query("proxy_cookie"), c.GetHeader("X-Cookie")); cookie != "" {
		req.Header.Set("Cookie", cookie)
	}

	// --- 执行请求 ---
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.String(http.StatusBadGateway, "Proxy Error: %v", err)
		return
	}
	defer resp.Body.Close()

	// --- 转发响应头 (Response Headers) ---
	// 只补尚未设置的头：CORS 中间件已设的 ACAO 优先，避免上游 CORS 头叠加成多值；
	// 源站专属的权限/SSL 控制头（HSTS/CSP/X-Frame-Options 等）不透传
	for k, vv := range resp.Header {
		if tools.IsHopByHop(k) || tools.IsOriginServerHeader(k) {
			continue
		}
		if c.Writer.Header().Get(k) != "" {
			continue
		}
		for _, v := range vv {
			c.Writer.Header().Add(k, v)
		}
	}

	// --- 重写 3xx 的 Location,让重定向留在代理内,避免裸 301 引发无限循环 ---
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		if loc := resp.Header.Get("Location"); loc != "" {
			carry := url.Values{}
			if v := tools.Or(c.Query("proxy_origin"), c.GetHeader("X-Origin")); v != "" {
				carry.Set("proxy_origin", v)
			}
			if v := tools.Or(c.Query("proxy_referer"), c.GetHeader("X-Referer")); v != "" {
				carry.Set("proxy_referer", v)
			}
			if v := tools.Or(c.Query("proxy_cookie"), c.GetHeader("X-Cookie")); v != "" {
				carry.Set("proxy_cookie", v)
			}
			c.Writer.Header().Set("Location", tools.RewriteLocation(loc, c.Request.Host, clientScheme, host, scheme, c.Request.URL.Path, search, carry))
		}
	}

	// 自定义 Header
	c.Writer.Header().Set("X-Proxy-Status", "success")
	c.Writer.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
	c.Writer.Header().Set("Timing-Allow-Origin", "*")

	// --- 返回响应内容 ---
	c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, nil)
}

// generateProxyGeneratorHTML 生成代理 URL 生成器页面
func generateProxyGeneratorHTML() string {
	return `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Proxy URL Generator - moonchan.xyz</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            padding: 20px;
        }
        
        .container {
            max-width: 900px;
            margin: 0 auto;
            background: white;
            border-radius: 12px;
            box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
            overflow: hidden;
        }
        
        .header {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 30px;
            text-align: center;
        }
        
        .header h1 {
            font-size: 28px;
            margin-bottom: 10px;
        }
        
        .header p {
            opacity: 0.9;
            font-size: 14px;
        }
        
        .content {
            padding: 30px;
        }
        
        .input-group {
            margin-bottom: 25px;
        }
        
        .input-group label {
            display: block;
            margin-bottom: 8px;
            font-weight: 600;
            color: #333;
            font-size: 14px;
        }
        
        .input-group input[type="text"] {
            width: 100%;
            padding: 12px 15px;
            border: 2px solid #e0e0e0;
            border-radius: 8px;
            font-size: 14px;
            transition: all 0.3s;
        }
        
        .input-group input[type="text"]:focus {
            outline: none;
            border-color: #667eea;
            box-shadow: 0 0 0 3px rgba(102, 126, 234, 0.1);
        }
        
        .input-group .hint {
            margin-top: 6px;
            font-size: 12px;
            color: #666;
        }
        
        .btn-generate {
            width: 100%;
            padding: 14px;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            border: none;
            border-radius: 8px;
            font-size: 16px;
            font-weight: 600;
            cursor: pointer;
            transition: transform 0.2s, box-shadow 0.2s;
        }
        
        .btn-generate:hover {
            transform: translateY(-2px);
            box-shadow: 0 10px 20px rgba(102, 126, 234, 0.3);
        }
        
        .btn-generate:active {
            transform: translateY(0);
        }
        
        .results {
            margin-top: 30px;
            display: none;
        }
        
        .results.show {
            display: block;
        }
        
        .result-card {
            background: #f8f9fa;
            border: 2px solid #e0e0e0;
            border-radius: 8px;
            padding: 20px;
            margin-bottom: 20px;
        }
        
        .result-card h3 {
            color: #667eea;
            margin-bottom: 12px;
            font-size: 16px;
            display: flex;
            align-items: center;
            gap: 8px;
        }
        
        .result-card h3 .badge {
            background: #667eea;
            color: white;
            padding: 2px 8px;
            border-radius: 4px;
            font-size: 11px;
            font-weight: 600;
        }
        
        .url-display {
            background: white;
            border: 1px solid #ddd;
            border-radius: 6px;
            padding: 12px;
            font-family: 'Courier New', monospace;
            font-size: 13px;
            word-break: break-all;
            margin-bottom: 10px;
            max-height: 150px;
            overflow-y: auto;
        }
        
        .btn-copy {
            padding: 8px 16px;
            background: #667eea;
            color: white;
            border: none;
            border-radius: 6px;
            font-size: 13px;
            cursor: pointer;
            transition: background 0.2s;
        }
        
        .btn-copy:hover {
            background: #5568d3;
        }
        
        .btn-copy.copied {
            background: #28a745;
        }
        
        .info-box {
            background: #e7f3ff;
            border-left: 4px solid #667eea;
            padding: 12px 15px;
            margin-bottom: 20px;
            border-radius: 4px;
            font-size: 13px;
            color: #333;
        }
        
        .auto-detect {
            background: #fff3cd;
            border-left: 4px solid #ffc107;
            padding: 10px 15px;
            margin-top: 10px;
            border-radius: 4px;
            font-size: 13px;
            display: none;
        }
        
        .auto-detect.show {
            display: block;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🔗 Proxy URL Generator</h1>
            <p>输入目标 URL，自动生成代理链接</p>
        </div>
        
        <div class="content">
            <div class="info-box">
                💡 提示：支持 Pixiv、微博等网站的图片代理，会自动添加合适的 Referer
            </div>
            
            <div class="input-group">
                <label for="targetUrl">目标 URL</label>
                <input type="text" id="targetUrl" placeholder="例如: https://i.pximg.net/img-master/img/2024/01/01/0000/123456.jpg">
                <div class="hint">输入完整的图片或其他资源 URL</div>
                <div class="auto-detect" id="autoDetect"></div>
            </div>
            
            <button class="btn-generate" onclick="generateUrls()">生成代理 URL</button>
            
            <div class="results" id="results">
                <div class="result-card">
                    <h3>
                        方式一：传统参数
                        <span class="badge">推荐</span>
                    </h3>
                    <div class="url-display" id="traditionalUrl"></div>
                    <button class="btn-copy" onclick="copyToClipboard('traditionalUrl', this)">复制</button>
                </div>
                
                <div class="result-card">
                    <h3>
                        方式二：Params 编码
                        <span class="badge">简洁</span>
                    </h3>
                    <div class="url-display" id="paramsUrl"></div>
                    <button class="btn-copy" onclick="copyToClipboard('paramsUrl', this)">复制</button>
                </div>
            </div>
        </div>
    </div>
    
    <script>
        // URL-safe Base64 编码
        function urlSafeBase64Encode(str) {
            return btoa(str)
                .replace(/\+/g, '-')
                .replace(/\//g, '_')
                .replace(/=/g, '');
        }
        
        // 自动检测域名并设置 Referer
        function detectReferer(url) {
            try {
                const urlObj = new URL(url);
                const hostname = urlObj.hostname;
                
                // Pixiv 图片
                if (hostname.match(/^i\.pximg\.net$/) || hostname.match(/^pximg\.net$/)) {
                    return 'https://www.pixiv.net/';
                }
                
                // 微博图片
                if (hostname.match(/^wx[1-4]\.sinaimg\.cn$/) || hostname.match(/^sinaimg\.cn$/)) {
                    return 'https://weibo.com/';
                }
                
                // Twitter/X 图片
                if (hostname.match(/^(pbs\.twimg\.com|ton\.twitter\.com)$/)) {
                    return 'https://twitter.com/';
                }
                
                return null;
            } catch (e) {
                return null;
            }
        }
        
        // 监听输入变化，自动检测
        document.getElementById('targetUrl').addEventListener('input', function(e) {
            const url = e.target.value.trim();
            const autoDetect = document.getElementById('autoDetect');
            
            if (url) {
                const referer = detectReferer(url);
                if (referer) {
                    autoDetect.textContent = '✨ 检测到 ' + new URL(url).hostname + '，将自动添加 Referer: ' + referer;
                    autoDetect.classList.add('show');
                } else {
                    autoDetect.classList.remove('show');
                }
            } else {
                autoDetect.classList.remove('show');
            }
        });
        
        // 生成 URL
        function generateUrls() {
            const targetUrl = document.getElementById('targetUrl').value.trim();
            
            if (!targetUrl) {
                alert('请输入目标 URL');
                return;
            }
            
            try {
                const urlObj = new URL(targetUrl);
                const host = urlObj.hostname;
                const path = urlObj.pathname + urlObj.search;
                const scheme = urlObj.protocol.replace(':', '');
                
                // 自动检测 Referer
                let referer = detectReferer(targetUrl);
                
                // 构建传统参数 URL
                let traditionalParams = [];
                traditionalParams.push('proxy_host=' + encodeURIComponent(host));
                traditionalParams.push('proxy_scheme=' + encodeURIComponent(scheme));
                if (referer) {
                    traditionalParams.push('proxy_referer=' + encodeURIComponent(referer));
                }
                
                // 保留原有的查询参数（排除代理专用参数）
                urlObj.searchParams.forEach((value, key) => {
                    if (!key.startsWith('proxy_') && key !== 'params') {
                        traditionalParams.push(encodeURIComponent(key) + '=' + encodeURIComponent(value));
                    }
                });
                
                const traditionalUrl = window.location.origin + path + '?' + traditionalParams.join('&');
                
                // 构建 Params 编码 URL
                const paramsObj = {
                    host: host,
                    scheme: scheme
                };
                if (referer) {
                    paramsObj.referer = referer;
                }
                
                const paramsJson = JSON.stringify(paramsObj);
                const paramsBase64 = urlSafeBase64Encode(paramsJson);
                
                // 保留原有的查询参数
                let otherParams = [];
                urlObj.searchParams.forEach((value, key) => {
                    if (!key.startsWith('proxy_') && key !== 'params') {
                        otherParams.push(encodeURIComponent(key) + '=' + encodeURIComponent(value));
                    }
                });
                
                let paramsUrl = window.location.origin + path + '?params=' + paramsBase64;
                if (otherParams.length > 0) {
                    paramsUrl += '&' + otherParams.join('&');
                }
                
                // 显示结果
                document.getElementById('traditionalUrl').textContent = traditionalUrl;
                document.getElementById('paramsUrl').textContent = paramsUrl;
                document.getElementById('results').classList.add('show');
                
            } catch (e) {
                alert('URL 格式错误: ' + e.message);
            }
        }
        
        // 复制到剪贴板
        function copyToClipboard(elementId, btn) {
            const text = document.getElementById(elementId).textContent;
            
            navigator.clipboard.writeText(text).then(() => {
                const originalText = btn.textContent;
                btn.textContent = '已复制!';
                btn.classList.add('copied');
                
                setTimeout(() => {
                    btn.textContent = originalText;
                    btn.classList.remove('copied');
                }, 2000);
            }).catch(err => {
                // 降级方案
                const textarea = document.createElement('textarea');
                textarea.value = text;
                document.body.appendChild(textarea);
                textarea.select();
                document.execCommand('copy');
                document.body.removeChild(textarea);
                
                btn.textContent = '已复制!';
                btn.classList.add('copied');
                setTimeout(() => {
                    btn.textContent = '复制';
                    btn.classList.remove('copied');
                }, 2000);
            });
        }
        
        // 支持回车键生成
        document.getElementById('targetUrl').addEventListener('keypress', function(e) {
            if (e.key === 'Enter') {
                generateUrls();
            }
        });
    </script>
</body>
</html>`
}
