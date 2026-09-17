/// <reference lib="webworker" />
/** @type {ServiceWorkerGlobalScope} */

const VERSION = 'V15-26.09.10'; 
const CACHE_NAME = `site-assets-${VERSION}`;

// ==================== 站点失效兜底 ====================
// 站点失效时弹「网噶了」页，正文是 /endpoint 给的新域名。
// 单独一个缓存名: CACHE_NAME 每次 VERSION 升级都会被 activate 删掉，
// 那样域名一换用户手里就没得看了。
const ENDPOINT_CACHE = 'endpoint-cache-v1';
const ENDPOINT_URL = '/endpoint';
const ENDPOINT_CHECK_KEY = '/endpoint-last-check';       // 记上次探测时间（成功失败都记）
const ENDPOINT_CHECK_INTERVAL = 24 * 60 * 60 * 1000;     // 每日只检测一次
const ENDPOINT_TIMEOUT = 4000;
const ENDPOINT_EMPTY_TEXT = '新地址暂时取不到，稍后刷新重试';

// 域名是否已被判定过期/被替换（内存 flag，SW 活着期间有效）
let domainExpired = false;

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
      // ENDPOINT_CACHE 不能删：升级后站点可能立刻就挂了，那时只能靠它
      keys.map(key => (key !== CACHE_NAME && key !== ENDPOINT_CACHE) ? caches.delete(key) : null)
    ))
      .then(() => self.clients.claim())
      // 站点还活着的时候先探一次：把 /endpoint 内容缓存下来，顺便确认域名是好的
      // （每天最多一次，重复激活不会重复打）
      .then(() => checkDomainDaily())
      .catch(() => {})
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

  // ========== 新增白名单 ==========
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

    // 每日检测一次（不挡本次导航，当天查过就跳过）。
    // 注意：用的是「上一次探测」的结论，所以域名恢复后要多花一次导航才放行
    event.waitUntil(checkDomainDaily());

    // 1. 判定过期 → 直接弹「网噶了」页面，不等网络
    //    - 日期规则：nmbyd3.top 过了 2026-05-31
    //    - 探测结论：域名过期后主机商 serve 的是别的页面（甚至 200），已确认不是我们的
    const isTargetDomain = hostname === EXPIRE_CONFIG.domainSuffix || hostname.endsWith(`.${EXPIRE_CONFIG.domainSuffix}`);
    const isExpired = Date.now() > new Date(EXPIRE_CONFIG.targetDate).getTime();

    if ((isTargetDomain && isExpired) || domainExpired) {
      console.warn(`[SW] 域名 ${hostname} 已过期，直接展示备用导航页`);
      event.respondWith(createOfflineResponse());
      return;
    }

    // 2. 普通导航请求（无拦截）
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
 * 普通导航请求：网络优先 -> 15s超时 -> 弹出缓存的 /endpoint
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
    console.warn('[SW] 主页面网络请求失败或超时，拦截并展示备用导航页:', error);
    // 网络层失败（DNS/连接重置/超时）→ 大概率是域名挂了，记下来让后续导航不用再干等 15 秒
    markExpired(`导航请求失败: ${error}`);
    return createOfflineResponse();
  }
}

/**
 * 合法域名校验：只认裸域名（可带 http(s):// 前缀和端口），至少两段、顶级域必须是字母
 * 必须真的按域名格式校验——松规则 [a-z0-9.-]+ 会放过 "parking" 这种没有点的单词。
 * @param {string} text
 * @returns {boolean}
 */
function isPlausibleEndpoint(text) {
  if (!text || text.length > 253) return false;

  const host = text
    .replace(/^https?:\/\//i, '')   // 可选 scheme
    .replace(/:\d{1,5}$/, '');      // 可选端口

  if (!host || host.length > 253) return false;

  const labels = host.split('.');
  if (labels.length < 2) return false;                        // 必须有点："parking" 不是域名

  const tld = labels[labels.length - 1];
  if (!/^[a-z]{2,63}$/i.test(tld)) return false;              // 顶级域必须是字母，顺带排除 IP

  // 每段 1-63 字符，字母数字开头结尾，中间允许连字符
  return labels.every((label) => /^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$/i.test(label));
}

/**
 * 只读缓存，不联网（站点挂掉时不该让用户等任何网络超时）
 * @returns {Promise<string>} 没有就返回空串
 */
async function readCachedEndpoint() {
  try {
    const cache = await caches.open(ENDPOINT_CACHE);
    const cached = await cache.match(ENDPOINT_URL);
    if (cached) {
      const text = (await cached.text()).trim();
      if (isPlausibleEndpoint(text)) return text;
    }
  } catch (e) {
    // 缓存读不了，交给调用方兜底
  }
  return '';
}

/**
 * 判定域名已过期/被替换。离线时不下这个结论（那是本地断网）
 * @param {string} reason
 * @returns {false} 方便 `return markExpired(...)` 直接用
 */
function markExpired(reason) {
  if (self.navigator && self.navigator.onLine === false) {
    console.warn(`[SW] ${reason}（当前无网络，暂不判定过期）`);
    return false;
  }
  if (!domainExpired) console.warn(`[SW] 判定域名已过期/被替换: ${reason}`);
  domainExpired = true;
  return false;
}

/** 把端点写进缓存 */
async function cacheEndpoint(text) {
  const cache = await caches.open(ENDPOINT_CACHE);
  await cache.put(ENDPOINT_URL, new Response(text, {
    headers: { 'Content-Type': 'text/plain; charset=utf-8' }
  }));
}

/**
 * 上次探测时间（0 = 从没探过）。存缓存不存内存：SW 空闲会被杀，存内存等于每次重启都重探
 * @returns {Promise<number>}
 */
async function lastCheckAt() {
  try {
    const cache = await caches.open(ENDPOINT_CACHE);
    const rec = await cache.match(ENDPOINT_CHECK_KEY);
    if (rec) return Number(await rec.text()) || 0;
  } catch (e) {
    // 读不到就当没探过
  }
  return 0;
}

/** 记下探测时间。成功失败都要记，否则探测失败时每次导航都会重探 */
async function markCheckedAt() {
  try {
    const cache = await caches.open(ENDPOINT_CACHE);
    await cache.put(ENDPOINT_CHECK_KEY, new Response(String(Date.now())));
  } catch (e) {
    // 记不上就算了，最坏情况是多探一次
  }
}

/**
 * 每日只检测一次：不到 24 小时直接返回，一次网络都不打
 * @returns {Promise<boolean|null>} 探测了就返回结果，跳过则返回 null
 */
async function checkDomainDaily() {
  const last = await lastCheckAt();
  const age = Date.now() - last;

  if (last && age < ENDPOINT_CHECK_INTERVAL) {
    console.log(`[SW] ${Math.round(age / 3600000)} 小时前探过，跳过`);
    return null;
  }

  const healthy = await checkDomain();
  await markCheckedAt();
  return healthy;
}

/**
 * 探测域名是否还在正常服务，顺带刷新缓存里的端点。
 * 三条判据（从便宜到贵），命中任意一条即判定过期：
 *   1. 状态码不是 2xx   2. 不是 text/plain   3. body 不是合法域名
 * @returns {Promise<boolean>} 域名是否正常
 */
async function checkDomain() {
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), ENDPOINT_TIMEOUT);

  try {
    const res = await fetch(`${ENDPOINT_URL}?t=${Date.now()}`, {
      signal: controller.signal,
      cache: 'no-store'
    });

    // 1. 状态码必须是 2xx
    if (!res.ok) return markExpired(`/endpoint 返回 ${res.status}`);

    // 2. 必须声明是纯文本
    const type = res.headers.get('content-type') || '';
    if (!type.includes('text/plain')) {
      return markExpired(`/endpoint 返回的不是纯文本 (${type || '无 content-type'})`);
    }

    // 3. body 必须是一个合法域名
    const text = (await res.text()).trim();
    if (!isPlausibleEndpoint(text)) {
      return markExpired(`/endpoint 拿不到合法域名: ${text.slice(0, 60) || '(空)'}`);
    }

    await cacheEndpoint(text);
    if (domainExpired) console.log('[SW] 域名已恢复正常，撤销过期判定');
    domainExpired = false;
    return true;
  } catch (e) {
    return markExpired(`/endpoint 请求失败: ${e}`);
  } finally {
    clearTimeout(timeoutId);
  }
}

/**
 * 「网噶了」页面：整页 HTML，正文就是去新域名
 * @param {string} endpoint 可能为空
 * @returns {string}
 */
function buildOfflinePage(endpoint) {
  const block = endpoint
    ? `<p>去下面看看吧：</p>
        <a href="https://${endpoint}">https://${endpoint}</a>`
    : `<p>${ENDPOINT_EMPTY_TEXT}</p>`;

  return `<!DOCTYPE html>
<html lang="zh">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>网噶了</title>
    <style>
        body { font-family: sans-serif; padding: 40px; text-align: center; background: #f5f5f5; }
        h1 { color: #333; }
        a { display: block; margin: 15px 0; color: #0066cc; text-decoration: none; word-break: break-all; }
        button { margin-top: 20px; padding: 10px 20px; }
    </style>
</head>
<body>
    <div style="background: white; padding: 30px; border-radius: 8px; max-width: 500px; margin: 50px auto;">
        <h1>网噶了</h1>
        ${block}
        <button onclick="location.reload()">重新检查</button>
    </div>
</body>
</html>`;
}

/**
 * 失效时的响应：「网噶了」HTML 页面
 * @returns {Promise<Response>}
 */
async function createOfflineResponse() {
  // 直接弹上次缓存的，不等网络
  let endpoint = await readCachedEndpoint();
  if (!endpoint) {
    // 冷启动：缓存里什么都没有（装上 SW 时站点就已经挂了）。当天还没查过才补查一次
    await checkDomainDaily();
    endpoint = await readCachedEndpoint();
  }

  console.warn(`[SW] 展示备用导航页: ${endpoint || '(缓存里没有端点)'}`);
  return new Response(buildOfflinePage(endpoint), {
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
