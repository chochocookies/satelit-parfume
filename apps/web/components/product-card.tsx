"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { Heart, ImageOff } from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { apiClient, type ProductListItem } from "@/lib/api-client";
import { useCustomerAuthStore } from "@/stores/customer-auth-store";
import { formatRupiah } from "@/lib/format";
import { cn } from "@/lib/utils";

// Section 16's product card: image, name, price, availability, and now
// (Phase 12) a wishlist heart — the one this file's own previous comment
// said would come later: "a heart that doesn't do anything yet is worse
// than no heart." It does something for a guest too: it sends them to
// log in rather than silently failing, which is still a real outcome
// rather than the dead-link problem that comment was about.
//
// "use client": the heart needs state and a click handler, which a
// server component can't have — everywhere this renders (shop grid,
// home page) already happily hosts client components as children, so
// this is a one-way, low-risk change.
export function ProductCard({ product }: { product: ProductListItem }) {
  const router = useRouter();
  const queryClient = useQueryClient();
  const customerAccessToken = useCustomerAuthStore((s) => s.accessToken);
  const customerHydrated = useCustomerAuthStore((s) => s.hasHydrated);
  const isCustomer = customerHydrated && !!customerAccessToken;

  const stockKnown = product.branch_stock !== undefined;
  const inStock = (product.branch_stock ?? 0) > 0;

  // Every ProductCard on a page queries the same ["wishlist"] key —
  // TanStack Query dedupes identical in-flight keys itself, so a shop
  // grid with 24 cards still only fires one request, not 24.
  const wishlistQuery = useQuery({
    queryKey: ["wishlist"],
    queryFn: apiClient.listWishlist,
    enabled: isCustomer,
    staleTime: 60_000,
  });
  const isWishlisted = wishlistQuery.data?.some((entry) => entry.product.id === product.id) ?? false;

  const toggleWishlist = useMutation({
    mutationFn: () =>
      isWishlisted ? apiClient.removeFromWishlist(product.id) : apiClient.addToWishlist(product.id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["wishlist"] }),
  });

  function handleHeartClick(e: React.MouseEvent) {
    // Stop the click from bubbling to the <Link> wrapping this card —
    // without this, tapping the heart would also navigate to the
    // product page underneath it.
    e.preventDefault();
    e.stopPropagation();
    if (!isCustomer) {
      router.push("/login");
      return;
    }
    toggleWishlist.mutate();
  }

  return (
    <Link
      href={`/product/${product.slug}`}
      className="group flex flex-col overflow-hidden rounded-2xl border border-line bg-surface transition hover:border-accent"
    >
      <div className="relative flex aspect-square items-center justify-center overflow-hidden bg-background">
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

        <button
          onClick={handleHeartClick}
          disabled={toggleWishlist.isPending}
          aria-label={isWishlisted ? "Hapus dari wishlist" : "Simpan ke wishlist"}
          aria-pressed={isWishlisted}
          className="absolute right-2 top-2 flex h-8 w-8 items-center justify-center rounded-full bg-background/80 text-ink-muted backdrop-blur transition hover:text-accent disabled:opacity-60"
        >
          <Heart className={cn("h-4 w-4", isWishlisted && "fill-accent text-accent")} />
        </button>
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
