"use client";

import { useEffect } from "react";

// Registration has to run client-side (navigator.serviceWorker is a
// browser API) — mounted once from the root layout, below. Silently
// no-ops in any browser without service worker support rather than
// throwing; a PWA install prompt just won't show there, same as before
// this existed.
export function ServiceWorkerRegistration() {
  useEffect(() => {
    if ("serviceWorker" in navigator) {
      navigator.serviceWorker.register("/sw.js").catch(() => {
        // Registration failing (unsupported browser quirk, dev-mode
        // HTTPS requirement not met, etc.) shouldn't be user-visible —
        // the app works identically either way, just without offline
        // caching or an install prompt.
      });
    }
  }, []);

  return null;
}
