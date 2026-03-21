/// <reference lib="webworker" />
/** @type {ServiceWorkerGlobalScope} */

const VERSION = 'V7-26.03.22'; // 更新版本号
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

// ==================== 配置区域 ====================

// 1. 域名过期拦截配置
const EXPIRE_CONFIG = {
  // 设定的截止日期 (格式建议为 YYYY-MM-DD 或带有 ISO 8601 时区的格式)
  // 你可以根据需要修改这个日期，例如 '2026-05-01T00:00:00+08:00'
  targetDate: '2026-05-31T23:59:59+08:00',
  // 匹配的根域名 (会自动匹配该域名及其所有子域名)
  domainSuffix: 'nmbyd3.top'
};

// 2. 图片超时代理配置 (新增)
const IMAGE_PROXY_CONFIG = {
  timeout: 5000, // 超时时间 5000 毫秒 (5秒)
  proxyBaseUrl: 'https://proxy.moonchan.xyz', // 代理源基础 URL
  targetDomains: ['ehgt.org', 'www.ehgt.org'] // 需要应用此规则的域名
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

  const url = new URL(req.url);

  // A. 导航请求（加载 HTML 页面）
  if (req.mode === 'navigate') {
    event.respondWith(handleNavigation(req));
    return;
  }

  // B. 图片 5s 超时代理 fallback (核心新增)
  // 匹配特定域名，或者通过 req.destination === 'image' 匹配所有图片
  if (IMAGE_PROXY_CONFIG.targetDomains.includes(url.hostname)) {
    event.respondWith(handleImageWithProxy(req));
    return;
  }

  // C. 静态资源缓存
  if (['script', 'style', 'font'].includes(req.destination) || req.url.match(/\.(js|css|woff2?)$/)) {
    event.respondWith(handleAsset(req));
  }
});

// ==================== 核心逻辑：图片代理 fallback ====================
async function handleImageWithProxy(req) {
  const url = new URL(req.url);
  const controller = new AbortController();

  // 设定 5 秒超时
  const timeoutId = setTimeout(() => controller.abort(), IMAGE_PROXY_CONFIG.timeout);

  try {
    // 1. 尝试请求原图
    const response = await fetch(req, {
      signal: controller.signal
    });

    clearTimeout(timeoutId);

    // 如果原图报 404, 403, 502 等非 2xx 状态码，主动抛出错误以触发备用源
    if (!response.ok && response.status !== 0) {
      throw new Error(`Original image failed with status: ${response.status}`);
    }

    return response;

  } catch (error) {
    // 2. 如果超时(AbortError)、DNS错误或状态码非 200，走代理逻辑
    clearTimeout(timeoutId);
    console.warn(`[SW] 图片加载超时或失败 (${error})，切换至代理源...`);

    // 构建代理 URL 
    // 例如: 原路径 /path/to/file?abc=1
    // 结合代理 baseUrl: https://proxy.moonchan.xyz/path/to/file?abc=1
    const proxyUrl = new URL(url.pathname + url.search, IMAGE_PROXY_CONFIG.proxyBaseUrl);

    // 追加代理所需的主机名参数
    // 结果: https://proxy.moonchan.xyz/path/to/file?abc=1&proxy_host=ehgt.org
    proxyUrl.searchParams.set('proxy_host', url.hostname);

    try {
      // 3. 请求代理源
      const proxyResponse = await fetch(proxyUrl.toString(), {
        // 保持原图片的请求模式（防止跨域污染），通常 img 标签为 no-cors 或 cors
        mode: req.mode === 'navigate' ? 'cors' : req.mode,
        credentials: req.credentials
      });
      return proxyResponse;
    } catch (proxyError) {
      // 4. 如果代理源也挂了，只能静默失败或返回 504 占位
      console.error(`[SW] 代理源图片也获取失败: ${proxyUrl.toString()}`);
      return new Response('', { status: 504, statusText: 'Gateway Timeout' });
    }
  }
}

// 处理页面导航：过期判断 -> 网络优先 -> 5秒超时熔断

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