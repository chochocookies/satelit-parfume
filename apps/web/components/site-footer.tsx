export function SiteFooter() {
  return (
    <footer className="border-t border-line px-6 py-12 text-sm text-ink-muted">
      <div className="mx-auto flex max-w-6xl flex-col items-center gap-4 text-center">
        <span className="font-display text-base text-ink">SATELIT PARFUME</span>
        <p className="max-w-md">
          Toko refill parfum. Belanja yang sudah berjalan hari ini lewat Instagram dan Shopee kami.
        </p>
        <div className="flex items-center gap-4">
          <a
            href="https://www.instagram.com/satelit_parfume/"
            target="_blank"
            rel="noreferrer"
            className="transition hover:text-accent"
          >
            Instagram
          </a>
          <span className="text-ink-muted/50">·</span>
          <a
            href="https://shopee.co.id/satelit_parfume"
            target="_blank"
            rel="noreferrer"
            className="transition hover:text-accent"
          >
            Shopee
          </a>
        </div>
        <p className="text-xs text-ink-muted/70">© {new Date().getFullYear()} Satelit Parfume</p>
      </div>
    </footer>
  );
}
