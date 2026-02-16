/// <reference lib="webworker" />
/** @type {ServiceWorkerGlobalScope} */

const VERSION = 'V5-26.02.16';
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

const CHECK_CONFIG = {
  interval: 10000,
  checkUrl: () => `/sw.js?t=${Date.now()}`,
  validString: "電波が届きません",
  isBlocking: false,
  navigationTimeout: 3000,
  // 记录已刷新的客户端，避免重复刷新
  refreshedClients: new Map()
};

// ==================== 生命周期 ====================

self.addEventListener('install', (e) => {
  console.log(`[SW] ${VERSION} Installing...`);
  self.skipWaiting();
});

self.addEventListener('activate', (e) => {
  e.waitUntil(
    caches.keys().then(keys => Promise.all(
      keys.map(key => key !== CACHE_NAME ? caches.delete(key) : null)
    )).then(() => self.clients.claim())
      .then(() => {
        console.log('[SW] 已控制所有客户端，启动后台检查');
        startPeriodicCheck();
      })
  );
});

// ==================== 后台定时检查（立 flag）====================

let checkTimer = null;

function startPeriodicCheck() {
  performCheck();
  if (checkTimer) clearInterval(checkTimer);
  checkTimer = setInterval(performCheck, CHECK_CONFIG.interval);
}

async function performCheck() {
  try {
    const isValid = await validateDomain();
    console.log(`[SW] 后台检查: ${isValid ? '正常' : '异常'}`);
    
    if (!isValid && !CHECK_CONFIG.isBlocking) {
      // 发现异常，立 flag
      CHECK_CONFIG.isBlocking = true;
      console.log('[SW] 立 flag: isBlocking = true');
      // 强制刷新所有已打开页面，让它们经过导航拦截
      // 26.02.16 不要这样用，其实可以是打开在后台缓存了看的。
      // await hardRefreshAllClients();
    } else if (isValid && CHECK_CONFIG.isBlocking) {
      // 恢复正常，撤 flag
      CHECK_CONFIG.isBlocking = false;
      CHECK_CONFIG.refreshedClients.clear();
      console.log('[SW] 撤 flag: isBlocking = false');
    }
  } catch (err) {
    console.error('[SW] 检查失败:', err);
    if (!CHECK_CONFIG.isBlocking) {
      CHECK_CONFIG.isBlocking = true;
      // 26.02.16 不要这样用，其实可以是打开在后台缓存了看的。
      // await hardRefreshAllClients();
    }
  }
}

async function validateDomain() {
  try {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), CHECK_CONFIG.navigationTimeout);
    
    const res = await fetch(CHECK_CONFIG.checkUrl(), {
      signal: controller.signal,
      cache: 'no-store'
    });
    clearTimeout(timeout);
    
    const text = await res.text();
    return text.includes(CHECK_CONFIG.validString);
  } catch (e) {
    return false;
  }
}

// 强制刷新所有客户端（让它们走导航拦截）
// 26.02.16 不要这样用，其实可以是打开在后台缓存了看的。
async function hardRefreshAllClients() {
  const clients = await self.clients.matchAll({
    type: 'window',
    includeUncontrolled: false
  });
  
  console.log(`[SW] 强制刷新 ${clients.length} 个客户端`);
  
  for (const client of clients) {
    const now = Date.now();
    const lastRefresh = CHECK_CONFIG.refreshedClients.get(client.id) || 0;
    
    // 5秒内不重复刷新同一个客户端
    if (now - lastRefresh < 5000) continue;
    
    try {
      await client.navigate(client.url);
      CHECK_CONFIG.refreshedClients.set(client.id, now);
      console.log(`[SW] 已刷新: ${client.url}`);
    } catch (e) {
      console.error(`[SW] 刷新失败:`, e);
    }
  }
}

// ==================== Fetch 拦截（关键逻辑）====================

self.addEventListener('fetch', (event) => {
  const req = event.request;
  
  if (!req.url.startsWith('http')) return;
  
  // A. 导航请求：只有立了 flag 才拦截，否则直接放行
  if (req.mode === 'navigate') {
    // 没有 flag，直接放行，不做任何检查
    if (!CHECK_CONFIG.isBlocking) {
      return;
    }
    
    // 有 flag，进行验证
    event.respondWith(handleBlockedNavigation(req));
    return;
  }
  
  // 26.02.16 不要这个
  // B. 阻断模式下拦截子资源（可选，加速失败）
  // if (CHECK_CONFIG.isBlocking && req.destination !== 'document') {
  //   event.respondWith(new Response('', {status: 503}));
  //   return;
  // }
  
  // C. 静态资源缓存
  // 26.02.16 改掉了image
  if (['script', 'style', 'font'].includes(req.destination) || 
      req.url.match(/\.(js|css|woff2?)$/)) {
    event.respondWith(handleAsset(req));
  }
});

/**
 * 处理被阻断的导航：实时验证，通过则放行，不通过则拦截
 */
async function handleBlockedNavigation(req) {
  console.log('[SW] flag 已立，验证导航:', req.url);
  
  try {
    const isValid = await validateDomain();
    
    if (isValid) {
      // 验证通过，撤 flag 并放行
      console.log('[SW] 验证通过，撤 flag 并放行');
      CHECK_CONFIG.isBlocking = false;
      CHECK_CONFIG.refreshedClients.clear();
      return fetch(req);
    } else {
      // 验证失败，保持 flag 并拦截
      console.log('[SW] 验证失败，拦截');
      return createOfflineResponse();
    }
    
  } catch (error) {
    // 验证出错，保守拦截
    console.log('[SW] 验证出错，拦截');
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
  
  const fetchPromise = fetch(req).then(res => {
    const type = res.headers.get('content-type') || '';
    const isHijacked = type.includes('text/html') && req.url.match(/\.(js|css)$/);
    
    if (res.ok && !isHijacked) {
      cache.put(req, res.clone());
    }
    return res;
  }).catch(() => cached);
  
  return cached || fetchPromise;
}