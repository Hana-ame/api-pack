const CACHE = 'ehviewer-v1';
const CACHE_FILES = ['/', '/index.html', '/manifest.json'];

self.addEventListener('install', e => {
  self.skipWaiting();
  e.waitUntil(caches.open(CACHE).then(c => c.addAll(CACHE_FILES)));
});

self.addEventListener('activate', e => {
  e.waitUntil(clients.claim());
});

self.addEventListener('fetch', e => {
  const req = e.request;
  if (req.mode === 'navigate') {
    e.respondWith(caches.match('/index.html').then(r => r || fetch(req)));
    return;
  }
  e.respondWith(
    caches.match(req).then(r => r || fetch(req).then(res => {
      if (res.ok && req.url.startsWith(self.location.origin)) {
        const clone = res.clone();
        caches.open(CACHE).then(c => c.put(req, clone));
      }
      return res;
    }))
  );
});
