/// <reference lib="webworker" />
/** @type {ServiceWorkerGlobalScope} */

const VERSION = 'V6-26.03.21'; // 更新了版本号以强制 SW 更新
const CACHE_NAME = `site-assets-${VERSION}`;

const OFFLINE_HTML_CONTENT = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>電波が届きません</title>
    <style>
        body { font-family: sans-serif; padding: 40px; text-align: center; background: #f5f5f5; }
        h1 { color: #333; }
        a { display: block; margin: 15px 0; color: #0066cc; text-decoration: none; }
        button { margin-top: 20px; padding: 10px 20px; }
    </style>
</head>
<body>
    <div style="background: white; padding: 30px; border-radius: 8px; max-width: 500px; margin: 50px auto;">
        <h1>電波が届きません</h1>
        <p>网噶了，去下面看看吧：</p>
        <a href="https://reminder.810114.xyz">https://reminder.810114.xyz</a>
        <a href="https://reminder.nmbyd3.top">https://reminder.nmbyd3.top</a>
        <button onclick="location.reload()">重新检查</button>
    </div>
</body>
</html>
`;

// ==================== 新增：过期域名拦截配置 ====================
const EXPIRE_CONFIG = {
  // 设定的截止日期 (格式建议为 YYYY-MM-DD 或带有 ISO 8601 时区的格式)
  // 你可以根据需要修改这个日期，例如 '2026-05-01T00:00:00+08:00'
  targetDate: '2026-05-31T23:59:59+08:00', 
  // 匹配的根域名 (会自动匹配该域名及其所有子域名)
  domainSuffix: 'nmbyd3.top'
};

// ==================== 生命周期 ====================

self.addEventListener('install', (e) => {
  console.log(`[SW] ${VERSION} Installing...`);
  self.skipWaiting();
});

self.addEventListener('activate', (e) => {
  console.log(`[SW] ${VERSION} activating...`);
  e.waitUntil(
    caches.keys().then(keys => Promise.all(
      keys.map(key => key !== CACHE_NAME ? caches.delete(key) : null)
    )).then(() => self.clients.claim())
  );
});

// ==================== Fetch 拦截 ====================

self.addEventListener('fetch', (event) => {
  const req = event.request;
  
  if (!req.url.startsWith('http')) return;
  
  // A. 导航请求（加载 HTML 页面）
  if (req.mode === 'navigate') {
    event.respondWith(handleNavigation(req));
    return;
  }
  
  // B. 静态资源缓存
  if (['script', 'style', 'font'].includes(req.destination) || req.url.match(/\.(js|css|woff2?)$/)) {
    event.respondWith(handleAsset(req));
  }
});

/**
 * 处理页面导航：过期判断 -> 网络优先 -> 5秒超时熔断
 */
async function handleNavigation(req) {
  const url = new URL(req.url);
  const hostname = url.hostname;

  // 1. ==================== 日期与域名拦截逻辑 ====================
  // 检查域名：是 nmbyd3.top 本身，或是以 .nmbyd3.top 结尾的子域名
  const isTargetDomain = hostname === EXPIRE_CONFIG.domainSuffix || hostname.endsWith(`.${EXPIRE_CONFIG.domainSuffix}`);
  // 检查时间：当前时间是否大于设定的过期时间
  const isExpired = Date.now() > new Date(EXPIRE_CONFIG.targetDate).getTime();

  if (isTargetDomain && isExpired) {
    console.warn(`[SW] 域名 ${hostname} 已超过设定日期 ${EXPIRE_CONFIG.targetDate}，直接展示错误页`);
    return createOfflineResponse(); // 直接返回错误页，不走任何网络请求
  }

  // 2. ==================== 正常的请求逻辑 ====================
  // 设置 5 秒超时，防止网络黑洞导致无限转圈白屏
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), 5000);

  try {
    const response = await fetch(req, {
      signal: controller.signal,
      cache: 'no-cache' 
    });
    
    clearTimeout(timeoutId);

    // 状态码如果不是 2xx（比如 502, 404, 403 等），展示错误页
    if (!response.ok) {
      console.warn(`[SW] 导航请求失败，状态码: ${response.status}，展示离线页`);
      return createOfflineResponse();
    }

    return response;

  } catch (error) {
    clearTimeout(timeoutId);
    console.warn('[SW] 网络请求报错或超时，拦截并展示离线页:', error);
    return createOfflineResponse();
  }
}

function createOfflineResponse() {
  return new Response(OFFLINE_HTML_CONTENT, {
    headers: {
      'Content-Type': 'text/html; charset=utf-8',
      'Cache-Control': 'no-store'
    }
  });
}

// ==================== 静态资源处理 ====================

async function handleAsset(req) {
  const cache = await caches.open(CACHE_NAME);
  const cached = await cache.match(req);
  
  if (cached) {
    fetchAndCache(req, cache);
    return cached;
  }
  
  try {
    const res = await fetch(req);
    if (res.ok) {
      const type = res.headers.get('content-type') || '';
      if (!type.includes('text/html')) {
        cache.put(req, res.clone());
      }
    }
    return res;
  } catch (err) {
    console.warn('[SW] 静态资源获取失败:', req.url);
    return new Response('', { status: 503, statusText: 'Service Unavailable' });
  }
}

async function fetchAndCache(req, cache) {
  try {
    const res = await fetch(req);
    if (res.ok) {
      const type = res.headers.get('content-type') || '';
      if (!type.includes('text/html')) {
        await cache.put(req, res.clone());
      }
    }
  } catch (e) {}
}