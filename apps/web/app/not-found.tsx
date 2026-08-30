import Link from "next/link";
import { Compass } from "lucide-react";

export default function NotFound() {
  return (
    <div className="flex min-h-[70vh] flex-col items-center justify-center gap-4 px-6 text-center">
      <div className="animate-fade-in-up flex flex-col items-center gap-4">
        <Compass className="h-10 w-10 text-accent" aria-hidden="true" />
        <div>
          <h1 className="font-display text-2xl text-ink">Halaman Tidak Ditemukan</h1>
          <p className="mt-2 max-w-sm text-sm text-ink-muted">Halaman yang Anda cari tidak ada atau sudah dipindahkan.</p>
        </div>
        <Link
          href="/"
          className="rounded-full bg-accent px-5 py-2.5 text-sm font-medium text-background transition hover:opacity-90"
        >
          Kembali ke Beranda
        </Link>
      </div>
    </div>
  );
}
