const VERSION = 'V2'; // Increment this to force an update
const CACHE_NAME = `site-assets-${VERSION}`;
const OFFLINE_URL = '/failed.html';

// 1. Install: Pre-cache the fallback page
self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => {
      // {cache: 'reload'} ensures we get the fresh version from server, not old browser cache
      return cache.add(new Request(OFFLINE_URL, { cache: 'reload' }));
    })
  );
  self.skipWaiting();
});

// 2. Activate: Clean up old versions and take control
self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((keys) => Promise.all(
      keys.map((key) => { if (key !== CACHE_NAME) return caches.delete(key); })
    ))
  );
  self.clients.claim();
});

// 3. Fetch: The Engine
self.addEventListener('fetch', (event) => {
  const req = event.request;

  // A. Handle Page Navigations (Showing failed.html if offline)
  if (req.mode === 'navigate') {
    event.respondWith(
      fetch(req).catch(() => caches.match(OFFLINE_URL))
    );
    return;
  }

  // B. Handle Assets (JS, CSS, and "import" modules)
  const isAsset = req.destination === 'script' || req.destination === 'style' ||
                  req.url.endsWith('.js') || req.url.endsWith('.css');

  if (isAsset) {
    event.respondWith(
      caches.open(CACHE_NAME).then((cache) => {
        return cache.match(req).then((cachedResponse) => {
          // Fetch from network and update cache in background (Stale-While-Revalidate)
          const fetchPromise = fetch(req).then((networkResponse) => {
            if (networkResponse.ok || networkResponse.status === 0) {
              cache.put(req, networkResponse.clone());
            }
            return networkResponse;
          }).catch(() => {}); // Fail silently if asset is missing and no network

          return cachedResponse || fetchPromise;
        });
      })
    );
  }
});