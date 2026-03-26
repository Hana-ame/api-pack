/// <reference lib="webworker" />
/** @type {ServiceWorkerGlobalScope} */

const VERSION = 'V11-26.03.22'; 
const CACHE_NAME = `site-assets-${VERSION}`;

// ==================== 自定义错误页面 ====================
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
        <button onclick="location.reload()">重新检查</button>
    </div>
</body>
</html>
`;

// ==================== 配置区域 ====================

// 1. 域名过期拦截配置
const EXPIRE_CONFIG = {
  targetDate: '2026-05-31T23:59:59+08:00', // 设定日期
  domainSuffix: 'nmbyd3.top'               // 匹配此域名及所有子域名
};

// 2. 图片超时代理配置
const IMAGE_PROXY_CONFIG = {
  timeout: 10000, // 5秒
  proxyBaseUrl: 'https://proxy.moonchan.xyz',
  targetDomains: ['ehgt.org', 'www.ehgt.org']
};

// 用于统筹遗留请求的控制器集合
const activeControllers = new Set();

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
  const hostname = url.hostname;

  // A. 导航请求（加载 HTML 页面）
  if (req.mode === 'navigate') {

    const isNewWindow = event.clientId === "";
    if (isNewWindow) {
      return;
    }
    
    // 1. 页面跳转前：果断中止上一页所有尚未完成的图片/脚本请求，释放网络通道
    if (activeControllers.size > 0) {
      console.log(`[SW] 侦测到页面跳转，释放 ${activeControllers.size} 个遗留后台连接...`);
      for (const controller of activeControllers) {
        controller.isNavigateAbort = true; 
        controller.abort();
      }
      activeControllers.clear();
    }
      // 1. 判断日期与域名拦截
      const isTargetDomain = hostname === EXPIRE_CONFIG.domainSuffix || hostname.endsWith(`.${EXPIRE_CONFIG.domainSuffix}`);
      const isExpired = Date.now() > new Date(EXPIRE_CONFIG.targetDate).getTime();
    
      if (isTargetDomain && isExpired) {
        // console.warn(`[SW] 域名 ${hostname} 已过期，直接展示备用导航页`);
        event.respondWith(handleNavigation(req));
      }
    // 2. 拦截并接管当前主页面的加载（支持超时、错误和过期判断）
    
    return;
  }

  // B. 图片 5s 超时代理 fallback
  if (IMAGE_PROXY_CONFIG.targetDomains.includes(url.hostname)) {
    event.respondWith(handleImageWithProxy(req));
    return;
  }
  
  // C. 静态资源缓存
  if (['script', 'style', 'font'].includes(req.destination) || req.url.match(/\.(js|css|woff2?)$/)) {
    event.respondWith(handleAsset(req));
  }
});

// ==================== 核心逻辑处理 ====================

/**
 * 导航请求处理：日期拦截 -> 网络优先 -> 5s超时 -> 返回离线页
 */
async function handleNavigation(req) {
  const url = new URL(req.url);
  const hostname = url.hostname;

  // 1. 判断日期与域名拦截
  const isTargetDomain = hostname === EXPIRE_CONFIG.domainSuffix || hostname.endsWith(`.${EXPIRE_CONFIG.domainSuffix}`);
  const isExpired = Date.now() > new Date(EXPIRE_CONFIG.targetDate).getTime();

  if (isTargetDomain && isExpired) {
    console.warn(`[SW] 域名 ${hostname} 已过期，直接展示备用导航页`);
    return createOfflineResponse();
  }

  // 2. 正常的请求逻辑 (设定 5s 超时防止无限白屏)
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), 15000);

  try {
    const response = await fetch(req, {
      signal: controller.signal,
      cache: 'no-cache' 
    });
    
    clearTimeout(timeoutId);

    // 状态码不是 2xx，服务器挂了，直接返回离线页
    if (!response.ok) {
      console.warn(`[SW] 主页面响应异常 (${response.status})，展示备用导航页`);
      return createOfflineResponse();
    }

    return response;

  } catch (error) {
    clearTimeout(timeoutId);
    console.warn('[SW] 主页面网络请求失败或超时，拦截并展示备用导航页:', error);
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

/**
 * 图片处理：带有请求统筹与代理切换
 */
async function handleImageWithProxy(req) {
  const url = new URL(req.url);
  const controller = new AbortController();
  activeControllers.add(controller);
  
  const timeoutId = setTimeout(() => controller.abort(), IMAGE_PROXY_CONFIG.timeout);

  try {
    const response = await fetch(req, { signal: controller.signal });
    clearTimeout(timeoutId);
    if (!response.ok && response.status !== 0) throw new Error(`Status: ${response.status}`);
    return response;

  } catch (error) {
    clearTimeout(timeoutId);
    
    // 如果是因为用户刷新页面/跳转而中止的请求，直接静默退出
    if (controller.isNavigateAbort) {
      return new Response('', { status: 499, statusText: 'Client Closed Request' });
    }

    console.warn(`[SW] 图片加载超时或失败 (${error})，切换至代理源...`);
    const proxyUrl = new URL(url.pathname + url.search, IMAGE_PROXY_CONFIG.proxyBaseUrl);
    proxyUrl.searchParams.set('proxy_host', url.hostname);

    const proxyController = new AbortController();
    activeControllers.add(proxyController);

    try {
      const proxyResponse = await fetch(proxyUrl.toString(), {
        mode: req.mode === 'navigate' ? 'cors' : req.mode,
        credentials: req.credentials,
        signal: proxyController.signal
      });
      return proxyResponse;
    } catch (proxyError) {
      if (proxyController.isNavigateAbort) return new Response('', { status: 499 });
      return new Response('', { status: 504, statusText: 'Gateway Timeout' });
    } finally {
      activeControllers.delete(proxyController);
    }

  } finally {
    activeControllers.delete(controller);
  }
}

/**
 * 静态资源处理
 */
async function handleAsset(req) {
  const cache = await caches.open(CACHE_NAME);
  const cached = await cache.match(req);
  
  if (cached) {
    fetchAndCache(req, cache);
    return cached;
  }
  
  const controller = new AbortController();
  activeControllers.add(controller);

  try {
    const res = await fetch(req, { signal: controller.signal });
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
  } finally {
    activeControllers.delete(controller);
  }
}

async function fetchAndCache(req, cache) {
  const controller = new AbortController();
  activeControllers.add(controller);
  try {
    const res = await fetch(req, { signal: controller.signal });
    if (res.ok) {
      const type = res.headers.get('content-type') || '';
      if (!type.includes('text/html')) {
        await cache.put(req, res.clone());
      }
    }
  } catch (e) {
  } finally {
    activeControllers.delete(controller);
  }
}
