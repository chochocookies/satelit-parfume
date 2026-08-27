"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { ArrowRight, MapPin } from "lucide-react";

import { apiClient } from "@/lib/api-client";
import { ProductCard } from "@/components/product-card";

// Real sections now (hero, catalog teaser, categories, branch locator),
// replacing Phase 1's placeholder. Still no invented brand copy — the
// hero line below describes what a "toko refill parfum" is generically,
// not a claim about Satelit Parfume specifically, since the real
// Instagram/Shopee visual identity still isn't reachable from here (see
// README's design notes). Fragrance finder (section 18) isn't here yet —
// it's a whole guided-quiz feature of its own, not part of Phase 5's
// scope in the phase table.
export default function Home() {
  const { data: products, isLoading: productsLoading } = useQuery({
    queryKey: ["products", { sort: "newest", limit: 8 }],
    queryFn: () => apiClient.listProducts({ sort: "newest", limit: 8 }),
  });

  const { data: categories } = useQuery({
    queryKey: ["categories"],
    queryFn: apiClient.listCategories,
  });

  const { data: branches } = useQuery({
    queryKey: ["branches", null],
    queryFn: () => apiClient.listBranches(),
  });

  return (
    <main>
      {/* Hero */}
      <section className="flex flex-col items-center gap-6 px-6 py-24 text-center">
        <span className="text-xs uppercase tracking-[0.3em] text-ink-muted">Toko Refill Parfum</span>
        <h1 className="font-display text-4xl tracking-wide text-ink sm:text-6xl">SATELIT PARFUME</h1>
        <p className="max-w-md text-sm text-ink-muted sm:text-base">
          Parfum isi ulang dengan pilihan aroma favorit, harga terjangkau untuk pemakaian sehari-hari.
        </p>
        <Link
          href="/shop"
          className="mt-2 flex items-center gap-2 rounded-full bg-accent px-6 py-3 text-sm font-medium text-background transition hover:opacity-90"
        >
          Belanja Sekarang
          <ArrowRight className="h-4 w-4" />
        </Link>
      </section>

      {/* Categories */}
      {categories && categories.length > 0 && (
        <section className="px-6 py-12">
          <div className="mx-auto max-w-6xl">
            <h2 className="font-display text-2xl text-ink">Kategori</h2>
            <div className="mt-6 flex flex-wrap gap-3">
              {categories.map((category) => (
                <Link
                  key={category.id}
                  href={`/shop?category=${category.slug}`}
                  className="rounded-full border border-line bg-surface px-5 py-2 text-sm text-ink transition hover:border-accent"
                >
                  {category.name}
                </Link>
              ))}
            </div>
          </div>
        </section>
      )}

      {/* Product showcase */}
      <section className="px-6 py-12">
        <div className="mx-auto max-w-6xl">
          <div className="flex items-center justify-between">
            <h2 className="font-display text-2xl text-ink">Produk Terbaru</h2>
            <Link href="/shop" className="text-sm text-accent transition hover:opacity-80">
              Lihat semua →
            </Link>
          </div>

          <div className="mt-6 grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
            {productsLoading &&
              Array.from({ length: 4 }).map((_, i) => (
                <div key={i} className="aspect-[3/4] animate-pulse rounded-2xl bg-surface" />
              ))}

            {!productsLoading && products?.items.length === 0 && (
              <p className="col-span-full py-12 text-center text-sm text-ink-muted">
                Katalog masih kosong. Impor produk terverifikasi dulu — lihat README.
              </p>
            )}

            {products?.items.map((product) => (
              <ProductCard key={product.id} product={product} />
            ))}
          </div>
        </div>
      </section>

      {/* Branch locator teaser */}
      <section className="border-t border-line px-6 py-12">
        <div className="mx-auto max-w-6xl">
          <h2 className="font-display text-2xl text-ink">Cabang Kami</h2>

          {branches && branches.length > 0 ? (
            <div className="mt-6 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {branches.slice(0, 3).map((branch) => (
                <div key={branch.id} className="rounded-2xl border border-line bg-surface p-5">
                  <div className="flex items-start gap-3">
                    <MapPin className="mt-0.5 h-4 w-4 shrink-0 text-accent" />
                    <div>
                      <p className="text-ink">{branch.name}</p>
                      {branch.city && <p className="text-sm text-ink-muted">{branch.city}</p>}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <p className="mt-6 text-sm text-ink-muted">
              Belum ada cabang terdaftar. Tambahkan lewat endpoint admin — lihat README bagian
              &quot;Trying branches and inventory&quot;.
            </p>
          )}
        </div>
      </section>
    </main>
  );
}
