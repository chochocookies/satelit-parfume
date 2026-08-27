import Link from "next/link";
import { ImageOff } from "lucide-react";

import type { ProductListItem } from "@/lib/api-client";
import { formatRupiah } from "@/lib/format";
import { cn } from "@/lib/utils";

// Section 16's product card: image, name, price, availability. No
// wishlist heart icon — that's Phase 12, and a heart that doesn't do
// anything yet is worse than no heart.
export function ProductCard({ product }: { product: ProductListItem }) {
  const stockKnown = product.branch_stock !== undefined;
  const inStock = (product.branch_stock ?? 0) > 0;

  return (
    <Link
      href={`/product/${product.slug}`}
      className="group flex flex-col overflow-hidden rounded-2xl border border-line bg-surface transition hover:border-accent"
    >
      <div className="flex aspect-square items-center justify-center overflow-hidden bg-background">
        {product.primary_image_url ? (
          // Plain <img>, not next/image: product images have no real
          // hosting domain configured yet (section 49 — object storage
          // isn't wired up), and none of the 23 imported products
          // currently have one at all. Revisit once that exists.
          // eslint-disable-next-line @next/next/no-img-element
          <img
            src={product.primary_image_url}
            alt={product.name}
            className="h-full w-full object-cover transition duration-300 group-hover:scale-105"
          />
        ) : (
          <ImageOff className="h-8 w-8 text-ink-muted" aria-hidden="true" />
        )}
      </div>

      <div className="flex flex-1 flex-col gap-1 p-4">
        {product.brand && <span className="text-xs text-ink-muted">{product.brand.name}</span>}
        <h3 className="font-display text-base text-ink">{product.name}</h3>
        <span className="mt-1 text-sm text-accent">{formatRupiah(product.price_from)}</span>

        {stockKnown && (
          <span className={cn("mt-2 text-xs", inStock ? "text-emerald-400" : "text-red-400")}>
            {inStock ? `✓ Tersedia (${product.branch_stock})` : "Stok habis di toko ini"}
          </span>
        )}
      </div>
    </Link>
  );
}
