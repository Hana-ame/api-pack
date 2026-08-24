/// <reference lib="webworker" />
/** @type {ServiceWorkerGlobalScope} */

const VERSION = 'V13-26.08.24'; 
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

// 1. 域名过期拦截配置（过期后直接拦截，不再提示）
const EXPIRE_CONFIG = {
  targetDate: '2026-05-31T23:59:59+08:00', // 过期时间点
  domainSuffix: 'nmbyd3.top'               // 匹配此域名及所有子域名
};

// 2. 迁移提示配置（在过期前的一段时间内，注入 alert 提醒）
const PROMPT_CONFIG = {
  startDate: '2026-05-01T00:00:00+08:00', // 开始提示的日期
  endDate: EXPIRE_CONFIG.targetDate,      // 提示截止日期（过期当天结束前都提示）
  targetDomain: '810114.xyz'              // 目标迁移域名
};

// 3. 图片超时代理配置
const IMAGE_PROXY_CONFIG = {
  timeout: 10000,
  proxyBaseUrl: 'https://proxy.moonchan.xyz',
  targetDomains: ['ehgt.org', 'www.ehgt.org']
};

// 用于统筹遗留请求的控制器集合
const activeControllers = new Set();

// ==================== 辅助函数：迁移提示 ====================

/**
 * 判断是否处于迁移提示时段（startDate <= 当前时间 < endDate）
 */
function isPromptPeriod() {
  const now = Date.now();
  const start = new Date(PROMPT_CONFIG.startDate).getTime();
  const end = new Date(PROMPT_CONFIG.endDate).getTime();
  return now >= start && now < end;
}

/**
 * 获取迁移目标域名
 * @param {string} hostname 
 * @returns {string|null} 目标域名，若不匹配则返回 null
 */
function getMigrationTarget(hostname) {
    if (hostname === "ex.nmbyd3.top") return "e.810114.xyz";
  if (hostname === EXPIRE_CONFIG.domainSuffix) {
    return PROMPT_CONFIG.targetDomain;
  }
  if (hostname.endsWith(`.${EXPIRE_CONFIG.domainSuffix}`)) {
    return hostname.replace(`.${EXPIRE_CONFIG.domainSuffix}`, `.${PROMPT_CONFIG.targetDomain}`);
  }
  return null;
}

/**
 * 在 HTML 响应中注入迁移提示脚本（每天仅弹一次）
 * @param {Response} res 
 * @param {string} targetHost 
 * @returns {Promise<Response>}
 */
async function injectPromptToResponse(res, targetHost) {
  // 仅处理成功的、未重定向的 HTML 响应
  if (res.ok && !res.redirected && res.headers.get('content-type')?.includes('text/html')) {
    const clonedRes = res.clone();
    try {
      const text = await clonedRes.text();
      const newHeaders = new Headers(res.headers);
      newHeaders.delete('content-length');
      newHeaders.delete('content-encoding');

      const PROMPT_SCRIPT = `
        <script>
          (function() {
            try {
              var today = new Date().toDateString();
              if (localStorage.getItem('nmbyd3_migration_prompt') !== today) {
                localStorage.setItem('nmbyd3_migration_prompt', today);
                alert('请迁移至 ✨ ${targetHost}.上不去请在匿名版反馈。\nhttps://opencode.ai/go?ref=G7T0RKWR9N\n支付宝可订阅，5刀首月，得60刀tokens\n发我订阅截图，视为对服务器捐赠5刀。');
              }
            } catch(e) {}
          })();
        </script>
      `;

      let injectedHtml = text;
      if (/(<head[^>]*>)/i.test(text)) {
        injectedHtml = text.replace(/(<head[^>]*>)/i, match => match + PROMPT_SCRIPT);
      } else if (/(<html[^>]*>)/i.test(text)) {
        injectedHtml = text.replace(/(<html[^>]*>)/i, match => match + PROMPT_SCRIPT);
      } else {
        injectedHtml = PROMPT_SCRIPT + text;
      }

      return new Response(injectedHtml, {
        status: res.status,
        statusText: res.statusText,
        headers: newHeaders
      });
    } catch (e) {
      // 注入失败，返回原始响应
      return res;
    }
  }
  return res;
}

/**
 * 获取响应并注入迁移提示（若需要）
 * @param {Request} req 
 * @param {string} targetHost 
 * @returns {Promise<Response>}
 */
async function fetchWithPrompt(req, targetHost) {
  try {
    const res = await fetch(req);
    return await injectPromptToResponse(res, targetHost);
  } catch (e) {
    // 网络错误，抛出由上层处理
    throw e;
  }
}

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

    // 1. 优先判断：域名是否已过期（直接拦截，不再提示）
    const isTargetDomain = hostname === EXPIRE_CONFIG.domainSuffix || hostname.endsWith(`.${EXPIRE_CONFIG.domainSuffix}`);
    const isExpired = Date.now() > new Date(EXPIRE_CONFIG.targetDate).getTime();

    if (isTargetDomain && isExpired) {
      console.warn(`[SW] 域名 ${hostname} 已过期，直接展示备用导航页`);
      event.respondWith(createOfflineResponse());
      return;
    }

    // 2. 未过期但处于提示时段 → 注入迁移提醒（正常加载页面 + alert）
    if (isTargetDomain && isPromptPeriod()) {
      const targetHost = getMigrationTarget(hostname);
      if (targetHost) {
        console.log(`[SW] 迁移提示时段，为 ${hostname} 注入提醒 → ${targetHost}`);
        // 使用带注入的专用处理函数（保留超时/错误降级）
        event.respondWith(handleNavigationWithPrompt(req, targetHost));
        return;
      }
    }

    // 3. 普通导航请求（无拦截，无提示）
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
 * 普通导航请求：网络优先 -> 5s超时 -> 离线页
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
    return createOfflineResponse();
  }
}

/**
 * 带迁移提示的导航请求：网络优先 -> 注入提示 -> 超时/错误降级到离线页
 * @param {Request} req 
 * @param {string} targetHost 
 */
async function handleNavigationWithPrompt(req, targetHost) {
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
    // 注入提示脚本
    return await injectPromptToResponse(response, targetHost);
  } catch (error) {
    clearTimeout(timeoutId);
    console.warn('[SW] 带提示的主页面请求失败，降级到离线页:', error);
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
