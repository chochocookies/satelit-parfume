// Phase 13: PWA. Scoped deliberately narrow — installability (this file
// + manifest.json + the icons alongside it) and just enough offline
// resilience to be worth calling a PWA, not a full offline-first
// rewrite of how the app fetches data. Phase 14 (Capacitor) is where
// this gets wrapped natively; this phase is the web-standards half.
//
// The one rule everything else here is built around: /api/ is NEVER
// cached, under any strategy. Stock counts, prices, cart contents, and
// order status all come from there — serving any of that from a cache,
// even for a few seconds, risks showing a shopper a price or an
// "in stock" that's already wrong. Only the static app shell (JS/CSS/
// icons/HTML) gets cached.
const CACHE_NAME = "satelit-parfume-shell-v1";

// Precached up front — everything else gets cached opportunistically on
// first fetch instead (see the fetch handler below), since Next.js's
// hashed JS/CSS chunk filenames change on every deploy and aren't known
// ahead of time.
const PRECACHE_URLS = ["/", "/offline", "/manifest.json", "/icon-192.png", "/icon-512.png"];

self.addEventListener("install", (event) => {
  event.waitUntil(
    caches
      .open(CACHE_NAME)
      .then((cache) => cache.addAll(PRECACHE_URLS))
      .then(() => self.skipWaiting()),
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) => Promise.all(keys.filter((key) => key !== CACHE_NAME).map((key) => caches.delete(key))))
      .then(() => self.clients.claim()),
  );
});

self.addEventListener("fetch", (event) => {
  const url = new URL(event.request.url);

  // Never intercept API calls — see this file's own top comment.
  if (url.pathname.startsWith("/api/")) {
    return;
  }

  // Only same-origin GET requests get the cache treatment. Everything
  // else (POST/PUT/DELETE, or a cross-origin request like Duitku's own
  // domain during payment) passes straight through untouched.
  if (event.request.method !== "GET" || url.origin !== self.location.origin) {
    return;
  }

  event.respondWith(
    caches.match(event.request).then((cached) => {
      const network = fetch(event.request)
        .then((response) => {
          // Only cache genuinely successful, basic (same-origin, not
          // opaque/redirected) responses — an error page or a redirect
          // response isn't something later visits should be served
          // back as if it were the real page.
          if (response.ok && response.type === "basic") {
            const copy = response.clone();
            caches.open(CACHE_NAME).then((cache) => cache.put(event.request, copy));
          }
          return response;
        })
        .catch(() => {
          if (cached) return cached;
          if (event.request.mode === "navigate") return caches.match("/offline");
          return Response.error();
        });

      // Stale-while-revalidate for the app shell: an already-cached
      // shell loads instantly, while a fresh copy keeps fetching in the
      // background for next time. This only ever governs the shell
      // (HTML/JS/CSS/icons) — actual product/cart/order data still
      // always comes from the live /api/ call this handler never
      // touches, so a fast repeat load here never means stale stock or
      // pricing.
      return cached || network;
    }),
  );
});
