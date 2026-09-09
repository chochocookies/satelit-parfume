import { WifiOff } from "lucide-react";

// sw.js falls back to this route on a failed navigation with nothing
// already cached for it — see that file's fetch handler. Static and
// tiny on purpose: it has to render from the service worker's own
// precache, with no guarantee any data-fetching would succeed anyway.
export default function OfflinePage() {
  return (
    <main className="mx-auto flex min-h-[60vh] max-w-md flex-col items-center justify-center px-6 text-center">
      <WifiOff className="h-8 w-8 text-ink-muted" aria-hidden="true" />
      <h1 className="mt-4 font-display text-xl text-ink">Kamu sedang offline</h1>
      <p className="mt-2 text-sm text-ink-muted">
        Halaman ini belum tersimpan untuk dibuka tanpa koneksi. Sambungkan kembali ke internet dan coba lagi.
      </p>
    </main>
  );
}
