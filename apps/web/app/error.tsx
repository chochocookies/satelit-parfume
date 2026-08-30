"use client";

import { useEffect } from "react";
import Link from "next/link";
import { RefreshCw, TriangleAlert } from "lucide-react";

// Next.js's own error boundary for this route segment — without this
// file, any uncaught exception (a rejected mutateAsync, a render error,
// etc.) falls through to Next's built-in default screen, which is
// unstyled and in English regardless of the rest of the site. This
// gives every such failure a page that at least looks like it belongs
// here, with a way back rather than a dead end.
export default function GlobalError({ error, reset }: { error: Error & { digest?: string }; reset: () => void }) {
  useEffect(() => {
    // Client-side errors never reach the backend's own logging (see
    // pkg/response.InternalError) — this is the equivalent for the
    // frontend: at least land in the browser console with the full
    // stack, not just this page's friendly summary.
    console.error(error);
  }, [error]);

  return (
    <div className="flex min-h-[70vh] flex-col items-center justify-center gap-4 px-6 text-center">
      <div className="animate-fade-in-up flex flex-col items-center gap-4">
        <TriangleAlert className="h-10 w-10 text-accent" aria-hidden="true" />
        <div>
          <h1 className="font-display text-2xl text-ink">Terjadi Kesalahan</h1>
          <p className="mt-2 max-w-sm text-sm text-ink-muted">
            Ada yang tidak berjalan semestinya di halaman ini. Coba muat ulang, atau kembali ke beranda.
          </p>
        </div>
        <div className="flex gap-3">
          <button
            onClick={reset}
            className="flex items-center gap-2 rounded-full bg-accent px-4 py-2.5 text-sm font-medium text-background transition hover:opacity-90"
          >
            <RefreshCw className="h-4 w-4" />
            Coba Lagi
          </button>
          <Link
            href="/"
            className="flex items-center gap-2 rounded-full border border-line px-4 py-2.5 text-sm text-ink transition hover:border-accent"
          >
            Ke Beranda
          </Link>
        </div>
      </div>
    </div>
  );
}
