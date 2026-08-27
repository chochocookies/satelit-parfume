import { Suspense } from "react";

import { ShopPageClient } from "@/components/shop-page-client";

// ShopPageClient uses useSearchParams (so a shared/bookmarked link like
// /shop?search=aqua pre-fills the search box) — Next.js requires that
// hook's consumer to sit under a Suspense boundary.
export default function ShopPage() {
  return (
    <Suspense fallback={<ShopFallback />}>
      <ShopPageClient />
    </Suspense>
  );
}

function ShopFallback() {
  return <div className="px-6 py-24 text-center text-ink-muted">Memuat toko…</div>;
}
