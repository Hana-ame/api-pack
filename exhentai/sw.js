/// <reference lib="webworker" />
/** @type {ServiceWorkerGlobalScope} */

const VERSION = 'V16-26.10.13';
const CACHE_NAME = `site-assets-${VERSION}`;

// ==================== 自定义错误页面 ====================
// 回档到 V13 的简洁逻辑：
//   - 导航请求网络优先，15s 超时
//   - 失败/超时 → 展示离线页（一次性，不设粘性标记）
//   - 域名过期（日期规则）→ 直接展示离线页
//   - 「重新检查」按钮真的清缓存 + 反注册 SW + 重新加载
const OFFLINE_HTML_CONTENT = `
<!DOCTYPE html>
<html lang="zh">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>網噶了</title>
    <style>
        body { font-family: sans-serif; padding: 40px; text-align: center; background: #f5f5f5; }
        h1 { color: #333; }
        a { display: block; margin: 15px 0; color: #0066cc; text-decoration: none; word-break: break-all; }
        button { margin-top: 20px; padding: 10px 20px; cursor: pointer; }
    </style>
</head>
<body>
    <div style="background: white; padding: 30px; border-radius: 8px; max-width: 500px; margin: 50px auto;">
        <h1>網噶了</h1>
        <p>网络请求失败或超时，可能是临时故障。</p>
        <a href="https://reminder.810114.xyz">https://reminder.810114.xyz</a>
        <button id="recheck" onclick="cleanAndReload()">重新检查</button>
        <p id="status" style="color:#888;font-size:12px;margin-top:10px;"></p>
    </div>
    <script>
    async function cleanAndReload() {
        var s = document.getElementById('status');
        try {
            s.textContent = '正在清理缓存...';
            // 1. 反注册旧 SW（确保不再被旧的粘性标记拦截）
            var regs = await navigator.serviceWorker.getRegistrations();
            for (var i = 0; i < regs.length; i++) {
                await regs[i].unregister();
            }
            // 2. 删除所有 cache storage
            var keys = await caches.keys();
            await Promise.all(keys.map(function(k){ return caches.delete(k); }));
            s.textContent = '已清理，重新加载...';
        } catch(e) {
            s.textContent = '清理失败: ' + e.message;
        }
        // 3. 重新加载页面（浏览器会注册一个干净的 SW）
        location.reload();
    }
    </script>
</body>
</html>
`;

// ==================== 配置区域 ====================

// 1. 域名过期拦截配置（过期后直接拦截，不再提示）
const EXPIRE_CONFIG = {
  targetDate: '2026-05-31T23:59:59+08:00', // 过期时间点
  domainSuffix: 'nmbyd3.top'               // 匹配此域名及所有子域名
};

// 2. 图片超时代理配置
const IMAGE_PROXY_CONFIG = {
  timeout: 10000,
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

  // ------------------- 导航请求（HTML 页面）-------------------

    
  if (req.mode === 'navigate') {

  // ========== 白名单 ==========
  const isImageRedirect = url.searchParams.get('redirect_to') === 'image';
  const isFullImg = /^\/fullimg(\/|$)/.test(url.pathname);
  const isArchiver = /^\/archiver\.php(\/|$)/.test(url.pathname);

  if (isImageRedirect || isFullImg || isArchiver) {
    return; // 直接放行，浏览器处理所有响应（包括 301/302）
  }
  // ================================

      
    // 释放之前页面残留的后台请求（优化体验）
    if (activeControllers.size > 0) {
      console.log(`[SW] 侦测到页面跳转，释放 ${activeControllers.size} 个遗留后台连接...`);
      for (const controller of activeControllers) {
        controller.isNavigateAbort = true;
        controller.abort();
      }
      activeControllers.clear();
    }

    // 判定过期 → 直接弹「網噶了」页面，不等网络
    const isTargetDomain = hostname === EXPIRE_CONFIG.domainSuffix || hostname.endsWith(`.${EXPIRE_CONFIG.domainSuffix}`);
    const isExpired = Date.now() > new Date(EXPIRE_CONFIG.targetDate).getTime();

    if (isTargetDomain && isExpired) {
      console.warn(`[SW] 域名 ${hostname} 已过期，直接展示备用导航页`);
      event.respondWith(createOfflineResponse());
      return;
    }

    // 普通导航请求：网络优先，不设粘性标记
    event.respondWith(handleNavigation(req));
    return;
  }

  // ------------------- 图片超时代理 -------------------
  if (IMAGE_PROXY_CONFIG.targetDomains.includes(url.hostname)) {
    event.respondWith(handleImageWithProxy(req));
    return;
  }

  // ------------------- 静态资源缓存 -------------------
  if (['script', 'style', 'font'].includes(req.destination) || req.url.match(/\.(js|css|woff2?)$/)) {
    event.respondWith(handleAsset(req));
  }
});

// ==================== 核心逻辑处理 ====================

/**
 * 普通导航请求：网络优先 -> 15s超时 -> 离线页
 * 
 * 关键设计（回档修正）：
 * - 不设粘性 domainExpired 标记
 * - 每次导航都真的尝试网络
 * - 失败只弹离线页一次，下次导航重新尝试
 * - 不会因为单次失败就永久屏蔽网络
 */
async function handleNavigation(req) {
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), 15000);

  try {
    const response = await fetch(req, {
      signal: controller.signal,
      cache: 'no-cache'
    });
    clearTimeout(timeoutId);
    if (!response.ok) {
      console.warn(`[SW] 主页面响应异常 (${response.status})，展示备用导航页`);
      return createOfflineResponse();
    }
    return response;
  } catch (error) {
    clearTimeout(timeoutId);
    console.warn('[SW] 主页面网络请求失败或超时，展示备用导航页:', error);
    // 不设粘性标记，下次导航重新尝试
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
 * 静态资源处理（缓存优先）
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
    // 静默失败
  } finally {
    activeControllers.delete(controller);
  }
}
