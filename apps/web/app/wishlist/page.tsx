"use client";

import Link from "next/link";
import { Heart } from "lucide-react";
import { useQuery } from "@tanstack/react-query";

import { apiClient } from "@/lib/api-client";
import { useCustomerAuthStore } from "@/stores/customer-auth-store";
import { ProductCard } from "@/components/product-card";

// Reuses ProductCard as-is for each saved product — its heart button is
// already a toggle, so "remove from wishlist" here is the same tap as
// "add" was on the shop grid, not a second interaction to build.
export default function WishlistPage() {
  const customerAccessToken = useCustomerAuthStore((s) => s.accessToken);
  const customerHydrated = useCustomerAuthStore((s) => s.hasHydrated);
  const isCustomer = customerHydrated && !!customerAccessToken;

  const wishlistQuery = useQuery({
    queryKey: ["wishlist"],
    queryFn: apiClient.listWishlist,
    enabled: isCustomer,
  });

  if (customerHydrated && !isCustomer) {
    return (
      <main className="mx-auto max-w-6xl px-6 py-12">
        <div className="mx-auto max-w-sm rounded-2xl border border-line bg-surface p-6 text-center">
          <Heart className="mx-auto h-6 w-6 text-ink-muted" aria-hidden="true" />
          <p className="mt-3 text-sm text-ink-muted">
            <Link href="/login" className="text-accent hover:underline">
              Masuk
            </Link>{" "}
            untuk melihat wishlist kamu.
          </p>
        </div>
      </main>
    );
  }

  return (
    <main className="mx-auto max-w-6xl px-6 py-12">
      <h1 className="font-display text-2xl text-ink">Wishlist</h1>

      {wishlistQuery.isLoading && <p className="mt-6 text-sm text-ink-muted">Memuat wishlist…</p>}

      {wishlistQuery.data && wishlistQuery.data.length === 0 && (
        <p className="mt-6 text-sm text-ink-muted">
          Belum ada produk tersimpan. Ketuk ikon hati di produk yang kamu suka untuk menyimpannya di sini.
        </p>
      )}

      {wishlistQuery.data && wishlistQuery.data.length > 0 && (
        <div className="mt-6 grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
          {wishlistQuery.data.map((entry) => (
            <ProductCard key={entry.product.id} product={entry.product} />
          ))}
        </div>
      )}
    </main>
  );
}
