/// <reference lib="webworker" />
/** @type {ServiceWorkerGlobalScope} */

/**
 * 
 * 配置区域
 */
const VERSION = 'V4-260121'; // 既是版本号，也是我们用于验证网站是否存活的“特定字符串”
const CACHE_NAME = `site-assets-${VERSION}`;

// 核心策略：将失败页面的 HTML 直接内嵌在 JS 变量中
// 优点：无需发起网络请求，不会因为域名过期导致 fail.html 也变成广告页，且无重定向问题
const OFFLINE_HTML_CONTENT = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=, initial-scale=1.0">
    <title>電波が届きません</title>
</head>
<body>
    网噶了，去<a href="https://x.810114.xyz">https://x.810114.xyz</a>看看吧

    
    <a href="https://reminder.810114.xyz">https://reminder.810114.xyz</a>
    <a href="https://reminder.nmbyd3.top">https://reminder.nmbyd3.top</a>
</body>
</html>

`;

// --------------------------------------------------------------------------

// 1. Install: 安装阶段
self.addEventListener('install', (event) => {
  // 这里的 log 可以帮助调试
  console.log(`[Service Worker] ${VERSION} Installing...`);

  // 强制跳过等待，让新 Service Worker 立即接管
  self.skipWaiting();
});

// 2. Activate: 激活与清理
self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((keys) => Promise.all(
      keys.map((key) => {
        if (key !== CACHE_NAME) {
          console.log(`[Service Worker] Clearing old cache: ${key}`);
          return caches.delete(key);
        }
      })
    ))
  );
  // 立即控制所有页面
  self.clients.claim();
});

// 3. Fetch: 核心拦截与检测引擎
self.addEventListener('fetch', (event) => {
  const req = event.request;

  // 【修复错误】：解决 'chrome-extension' is unsupported 报错
  // 只有 http 或 https 协议的请求才允许被 Service Worker 处理和缓存
  if (!req.url.startsWith('http')) {
    return;
  }

  // A. 处理页面跳转 (Navigation) - 也就是用户在浏览器地址栏输入网址或点击链接时
  if (req.mode === 'navigate') {
    event.respondWith(handleNavigation(req));
    return;
  }

  // B. 处理静态资源 (JS, CSS, Images 等)
  const isAsset = req.destination === 'script' || req.destination === 'style' ||
    req.destination === 'image' || req.url.match(/\.(js|css|png|jpg|jpeg|gif)$/);

  if (isAsset) {
    event.respondWith(handleAsset(req));
  }
});

/**
 * 核心逻辑：处理页面导航请求
 * 包含域名健康度检查
 */
async function handleNavigation(req) {
  try {
    // 1. 尝试从网络获取目标页面
    const networkResponse = await fetch(req);

    // 2. 复制一份响应以进行检查（response流只能被读取一次）
    const clonedRes = networkResponse.clone();

    // 3. 初步判断：如果状态码是 200 且是 HTML，我们需要警惕是否是“域名停靠/过期”广告页
    const contentType = clonedRes.headers.get('content-type') || '';
    if (networkResponse.ok && contentType.includes('text/html')) {

      // 4. 执行【站点存活验证】：
      // 域名过期后，通常所有 URL（包括 /sw.js）都会返回同一个 HTML 广告页。
      // 我们请求 Service Worker 文件本身，并检查它是否包含特定的版本号字符串。
      // 加上时间戳 ?t= 防止浏览器缓存验证请求。
      const validationUrl = `/sw.js?t=${Date.now()}`;

      try {
        const checkRes = await fetch(validationUrl);
        const checkText = await checkRes.text();

        // 【验证逻辑】：
        // 如果 /sw.js 的内容里不包含我们定义的 VERSION 字符串 (例如 'V4')，
        // 或者返回的内容竟然是 HTML (checkText.includes('<!DOCTYPE html>'))，
        // 说明域名已失效，返回的是广告页。
        const isValidSite = checkText.includes("電波が届きません");

        if (!isValidSite) {
          console.warn('[SW 检测] 域名校验失败（/sw.js 内容异常），判定为域名过期/劫持。');
          return createOfflineResponse();
        }

      } catch (e) {
        // 如果连校验请求都发不出去，可能是断网，或者是跨域限制（通常 sw.js 同域不会跨域）
        // 这里可以选择放行或保守处理，这里选择放行网络响应
        console.warn('[SW 检测] 校验请求出错，保持原响应', e);
      }
    }

    // 5. 如果一切正常，返回网络请求
    return networkResponse;

  } catch (error) {
    // 6. 网络完全断开（Fetch 抛出异常），直接返回内嵌的离线页
    console.log('[SW] Network failed, serving offline page.');
    return createOfflineResponse();
  }
}

/**
 * 处理静态资源 (Stale-while-revalidate 策略)
 * 增加了防止错误缓存的逻辑
 */
async function handleAsset(req) {
  const cache = await caches.open(CACHE_NAME);
  const cachedResponse = await cache.match(req);

  // 网络请求逻辑
  const fetchPromise = fetch(req).then((networkResponse) => {
    // 【关键】：防止域名过期后的 HTML 覆盖了本地缓存的真实 JS/CSS
    // 如果请求的是 .js/.css，但返回的 Content-Type 是 text/html，说明被劫持了
    const type = networkResponse.headers.get('content-type') || '';
    const isHijacked = type.includes('text/html');

    if (networkResponse.ok && !isHijacked) {
      // 只有正常的资源才写入缓存
      // 再次检查 scheme，防止报错
      if (req.url.startsWith('http')) {
        cache.put(req, networkResponse.clone());
      }
    }
    return networkResponse;
  }).catch((err) => {
    // 网络失败时，不做处理，后面会返回缓存
    return null;
  });

  // 如果有缓存，优先返回缓存，同时后台更新；如果没缓存，等待网络
  return cachedResponse || fetchPromise;
}

/**
 * 辅助函数：构造内嵌的离线页面响应
 */
function createOfflineResponse() {
  return new Response(OFFLINE_HTML_CONTENT, {
    headers: {
      'Content-Type': 'text/html; charset=utf-8',
      'Cache-Control': 'no-store' // 确保失败页不被浏览器强缓存
    }
  });
}