"use client";

import { useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import { useQuery } from "@tanstack/react-query";

import { apiClient, type ProductListParams } from "@/lib/api-client";
import { useBranchStore } from "@/stores/branch-store";
import { ProductCard } from "@/components/product-card";

type SortValue = NonNullable<ProductListParams["sort"]>;

const SORT_OPTIONS: { value: SortValue; label: string }[] = [
  { value: "", label: "Nama A-Z" },
  { value: "newest", label: "Terbaru" },
  { value: "price_asc", label: "Harga: Rendah ke Tinggi" },
  { value: "price_desc", label: "Harga: Tinggi ke Rendah" },
];

export function ShopPageClient() {
  const searchParams = useSearchParams();

  const [search, setSearch] = useState(searchParams.get("search") ?? "");
  const [debouncedSearch, setDebouncedSearch] = useState(search);
  const [category, setCategory] = useState(searchParams.get("category") ?? "");
  const [sort, setSort] = useState<SortValue>((searchParams.get("sort") as SortValue) || "");
  const [page, setPage] = useState(1);

  const branchSlug = useBranchStore((s) => s.selectedBranch?.slug);

  // Debounce so typing doesn't hit the API on every keystroke.
  useEffect(() => {
    const timeout = setTimeout(() => setDebouncedSearch(search), 400);
    return () => clearTimeout(timeout);
  }, [search]);

  // Any filter change starts back at page 1 — staying on page 4 of a
  // now-3-page result set would just show an empty page. Adjusted during
  // render (React's recommended pattern for "reset state when an input
  // changes") rather than in a useEffect, which would cause an extra,
  // visibly-flashing render pass after the one that already updated the
  // filter.
  const [prevFilterKey, setPrevFilterKey] = useState({ debouncedSearch, category, sort, branchSlug });
  if (
    prevFilterKey.debouncedSearch !== debouncedSearch ||
    prevFilterKey.category !== category ||
    prevFilterKey.sort !== sort ||
    prevFilterKey.branchSlug !== branchSlug
  ) {
    setPrevFilterKey({ debouncedSearch, category, sort, branchSlug });
    setPage(1);
  }

  const { data: categories } = useQuery({
    queryKey: ["categories"],
    queryFn: apiClient.listCategories,
  });

  const { data, isLoading, isError } = useQuery({
    queryKey: ["products", { search: debouncedSearch, category, sort, page, branchSlug }],
    queryFn: () =>
      apiClient.listProducts({
        search: debouncedSearch || undefined,
        category: category || undefined,
        sort: sort || undefined,
        branch: branchSlug,
        page,
      }),
  });

  return (
    <main className="mx-auto max-w-6xl px-6 py-12">
      <h1 className="font-display text-3xl text-ink">Belanja</h1>

      <div className="mt-6 flex flex-col gap-3 sm:flex-row sm:items-center">
        <input
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Cari nama parfum…"
          className="flex-1 rounded-full border border-line bg-surface px-4 py-2 text-sm text-ink placeholder:text-ink-muted focus:border-accent focus:outline-none"
        />

        <select
          value={category}
          onChange={(e) => setCategory(e.target.value)}
          className="rounded-full border border-line bg-surface px-4 py-2 text-sm text-ink focus:border-accent focus:outline-none"
        >
          <option value="">Semua Kategori</option>
          {categories?.map((c) => (
            <option key={c.id} value={c.slug}>
              {c.name}
            </option>
          ))}
        </select>

        <select
          value={sort}
          onChange={(e) => setSort(e.target.value as SortValue)}
          className="rounded-full border border-line bg-surface px-4 py-2 text-sm text-ink focus:border-accent focus:outline-none"
        >
          {SORT_OPTIONS.map((opt) => (
            <option key={opt.value} value={opt.value}>
              {opt.label}
            </option>
          ))}
        </select>
      </div>

      {!branchSlug && (
        <p className="mt-4 text-xs text-ink-muted">Pilih toko di atas untuk lihat ketersediaan stok.</p>
      )}

      <div className="mt-8">
        {isLoading && <GridSkeleton />}

        {isError && (
          <p className="py-16 text-center text-sm text-ink-muted">
            Gagal memuat produk. Coba lagi sebentar lagi.
          </p>
        )}

        {!isLoading && !isError && data?.items.length === 0 && (
          <p className="py-16 text-center text-sm text-ink-muted">
            Tidak ada produk ditemukan. Coba kata kunci lain, atau impor katalog dulu — lihat README.
          </p>
        )}

        {!isLoading && data && data.items.length > 0 && (
          <>
            <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
              {data.items.map((item) => (
                <ProductCard key={item.id} product={item} />
              ))}
            </div>

            {data.total_pages > 1 && (
              <div className="mt-8 flex items-center justify-center gap-3 text-sm">
                <button
                  disabled={page <= 1}
                  onClick={() => setPage((p) => p - 1)}
                  className="rounded-full border border-line px-4 py-2 text-ink transition disabled:opacity-40"
                >
                  Sebelumnya
                </button>
                <span className="text-ink-muted">
                  Halaman {data.page} dari {data.total_pages}
                </span>
                <button
                  disabled={page >= data.total_pages}
                  onClick={() => setPage((p) => p + 1)}
                  className="rounded-full border border-line px-4 py-2 text-ink transition disabled:opacity-40"
                >
                  Berikutnya
                </button>
              </div>
            )}
          </>
        )}
      </div>
    </main>
  );
}

function GridSkeleton() {
  return (
    <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
      {Array.from({ length: 8 }).map((_, i) => (
        <div key={i} className="aspect-[3/4] animate-pulse rounded-2xl bg-surface" />
      ))}
    </div>
  );
}
