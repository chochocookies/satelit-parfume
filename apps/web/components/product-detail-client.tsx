"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ImageOff, Minus, Plus } from "lucide-react";

import { apiClient } from "@/lib/api-client";
import { useBranchStore } from "@/stores/branch-store";
import { useCart } from "@/hooks/use-cart";
import { formatRupiah } from "@/lib/format";
import { cn } from "@/lib/utils";
import { ProductReviews } from "@/components/product-reviews";

export function ProductDetailClient({ slug }: { slug: string }) {
  const [quantity, setQuantity] = useState(1);
  const selectedBranch = useBranchStore((s) => s.selectedBranch);
  const { addItem } = useCart();

  const {
    data: product,
    isLoading,
    isError,
  } = useQuery({
    queryKey: ["product", slug, selectedBranch?.slug],
    queryFn: () => apiClient.getProduct(slug, selectedBranch?.slug),
  });

  if (isLoading) {
    return <div className="mx-auto max-w-5xl px-6 py-24 text-center text-ink-muted">Memuat produk…</div>;
  }

  if (isError || !product) {
    return (
      <div className="mx-auto max-w-5xl px-6 py-24 text-center">
        <p className="text-ink-muted">Produk tidak ditemukan.</p>
      </div>
    );
  }

  const primaryImage = product.images.find((img) => img.is_primary) ?? product.images[0];
  const price =
    product.availability?.price ??
    (product.variants.length > 0 ? Math.min(...product.variants.map((v) => v.base_price)) : 0);
  const availableStock = product.availability?.available_stock ?? 0;
  const inStock = availableStock > 0;
  // Almost every product has exactly one variant (see the product_variants
  // migration comment — no fake size lineups), so there's no variant
  // picker yet; this just uses the one that's there.
  const variant = product.variants[0];

  function handleAddToCart() {
    if (!selectedBranch || !variant) return;
    addItem.mutate(
      { product_variant_id: variant.id, branch_id: selectedBranch.id, quantity },
      { onSuccess: () => setQuantity(1) },
    );
  }

  return (
    <main className="mx-auto max-w-5xl px-6 py-12">
      <div className="grid gap-10 sm:grid-cols-2">
        <div className="flex aspect-square items-center justify-center overflow-hidden rounded-2xl border border-line bg-surface">
          {primaryImage ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img src={primaryImage.image_url} alt={product.name} className="h-full w-full object-cover" />
          ) : (
            <ImageOff className="h-12 w-12 text-ink-muted" aria-hidden="true" />
          )}
        </div>

        <div className="flex flex-col gap-3">
          {product.brand && <span className="text-sm text-ink-muted">{product.brand.name}</span>}
          <h1 className="font-display text-3xl text-ink">{product.name}</h1>
          <span className="text-2xl text-accent">{formatRupiah(price)}</span>

          {product.category && (
            <span className="w-fit rounded-full border border-line px-3 py-1 text-xs text-ink-muted">
              {product.category.name}
            </span>
          )}

          <div className="mt-2 rounded-xl border border-line bg-surface p-4 text-sm">
            {product.availability ? (
              <>
                <p className="text-ink-muted">Tersedia di</p>
                <p className="text-ink">{product.availability.branch_name}</p>
                <p className={cn("mt-1", inStock ? "text-emerald-400" : "text-red-400")}>
                  {inStock ? `✓ ${availableStock} tersisa` : "Stok habis di toko ini"}
                </p>
              </>
            ) : (
              <p className="text-ink-muted">Pilih toko di bagian atas halaman untuk lihat ketersediaan stok.</p>
            )}
          </div>

          {/* Add to cart — needs a selected branch, since branch_id is
              required on every cart item (section 20: a cart belongs to
              one branch). No branch selected means nothing to add
              against yet, so the prompt takes the button's place instead
              of a button that would just fail. */}
          {selectedBranch && variant ? (
            <div className="mt-2 flex items-center gap-3">
              <div className="flex items-center gap-2 rounded-full border border-line px-2 py-1">
                <button
                  onClick={() => setQuantity((q) => Math.max(1, q - 1))}
                  disabled={quantity <= 1}
                  aria-label="Kurangi jumlah"
                  className="flex h-7 w-7 items-center justify-center rounded-full text-ink transition disabled:opacity-40"
                >
                  <Minus className="h-3 w-3" />
                </button>
                <span className="w-6 text-center text-sm text-ink">{quantity}</span>
                <button
                  onClick={() => setQuantity((q) => Math.min(availableStock, q + 1))}
                  disabled={!inStock || quantity >= availableStock}
                  aria-label="Tambah jumlah"
                  className="flex h-7 w-7 items-center justify-center rounded-full text-ink transition disabled:opacity-40"
                >
                  <Plus className="h-3 w-3" />
                </button>
              </div>

              <button
                onClick={handleAddToCart}
                disabled={!inStock || addItem.isPending}
                className="flex-1 rounded-full bg-accent px-6 py-3 text-sm font-medium text-background transition hover:opacity-90 disabled:opacity-40"
              >
                {addItem.isPending ? "Menambahkan…" : inStock ? "Tambah ke Keranjang" : "Stok Habis"}
              </button>
            </div>
          ) : (
            <p className="mt-2 text-sm text-ink-muted">Pilih toko di bagian atas halaman untuk mulai belanja.</p>
          )}

          {addItem.isError && (
            <p className="text-xs text-red-400">
              {addItem.error instanceof Error ? addItem.error.message : "Gagal menambahkan ke keranjang."}
            </p>
          )}

          {product.description && (
            <p className="whitespace-pre-line text-sm text-ink-muted">{product.description}</p>
          )}
          {product.size && <p className="text-sm text-ink-muted">Ukuran: {product.size}</p>}
        </div>
      </div>

      <ProductReviews slug={slug} />
    </main>
  );
}
