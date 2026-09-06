"use client";

import Link from "next/link";
import { Award, Heart, Package } from "lucide-react";
import { useQuery } from "@tanstack/react-query";

import { apiClient } from "@/lib/api-client";
import { useCustomerAuthStore } from "@/stores/customer-auth-store";
import { formatRupiah } from "@/lib/format";
import { ProductCard } from "@/components/product-card";

// Riwayat Pesanan (order history) lives here, not on its own page —
// ListMine/GetMine have been sitting on the backend since orders' own
// earlier work with no frontend ever reading them. Membership and
// Wishlist keep their own full pages (already built) and just get a
// summary card + preview here, so this stays a genuine overview rather
// than three full pages duplicated onto one.
const statusLabel: Record<string, string> = {
  PENDING_PAYMENT: "Menunggu Pembayaran",
  PAID: "Sudah Dibayar",
  PROCESSING: "Diproses",
  PACKED: "Dikemas",
  READY_FOR_PICKUP: "Siap Diambil",
  SHIPPED: "Dikirim",
  DELIVERED: "Terkirim",
  COMPLETED: "Selesai",
  CANCELLED: "Dibatalkan",
  REFUNDED: "Direfund",
  EXPIRED: "Kedaluwarsa",
};

const statusColor: Record<string, string> = {
  PENDING_PAYMENT: "text-amber-400",
  CANCELLED: "text-red-400",
  REFUNDED: "text-red-400",
  EXPIRED: "text-red-400",
  COMPLETED: "text-emerald-400",
};

const tierLabel: Record<string, string> = { BRONZE: "Bronze", SILVER: "Silver", GOLD: "Gold" };

export default function AccountPage() {
  const customer = useCustomerAuthStore((s) => s.customer);
  const customerAccessToken = useCustomerAuthStore((s) => s.accessToken);
  const customerHydrated = useCustomerAuthStore((s) => s.hasHydrated);
  const isCustomer = customerHydrated && !!customerAccessToken;

  const ordersQuery = useQuery({ queryKey: ["my-orders"], queryFn: apiClient.listMyOrders, enabled: isCustomer });
  const membershipQuery = useQuery({
    queryKey: ["membership"],
    queryFn: apiClient.getMembershipStatus,
    enabled: isCustomer,
  });
  const wishlistQuery = useQuery({ queryKey: ["wishlist"], queryFn: apiClient.listWishlist, enabled: isCustomer });

  if (customerHydrated && !isCustomer) {
    return (
      <main className="mx-auto max-w-4xl px-6 py-12">
        <div className="mx-auto max-w-sm rounded-2xl border border-line bg-surface p-6 text-center">
          <p className="text-sm text-ink-muted">
            <Link href="/login" className="text-accent hover:underline">
              Masuk
            </Link>{" "}
            untuk melihat akun kamu.
          </p>
        </div>
      </main>
    );
  }

  return (
    <main className="mx-auto max-w-4xl px-6 py-12">
      <h1 className="font-display text-2xl text-ink">Halo, {customer?.name.split(" ")[0]}</h1>
      <p className="mt-1 text-sm text-ink-muted">{customer?.email}</p>

      <div className="mt-8 grid gap-4 sm:grid-cols-3">
        <Link
          href="/membership"
          className="rounded-2xl border border-line bg-surface p-5 transition hover:border-accent"
        >
          <div className="flex items-center gap-2 text-ink-muted">
            <Award className="h-4 w-4" />
            <span className="text-xs">Membership</span>
          </div>
          <p className="mt-2 font-display text-lg text-ink">
            {membershipQuery.data ? (tierLabel[membershipQuery.data.tier] ?? membershipQuery.data.tier) : "—"}
          </p>
          <p className="text-xs text-ink-muted">
            {membershipQuery.data ? `${formatRupiah(membershipQuery.data.total_spent)} total belanja` : "Memuat…"}
          </p>
        </Link>

        <Link href="/wishlist" className="rounded-2xl border border-line bg-surface p-5 transition hover:border-accent">
          <div className="flex items-center gap-2 text-ink-muted">
            <Heart className="h-4 w-4" />
            <span className="text-xs">Wishlist</span>
          </div>
          <p className="mt-2 font-display text-lg text-ink">{wishlistQuery.data?.length ?? "—"} produk</p>
          <p className="text-xs text-ink-muted">Tersimpan untuk nanti</p>
        </Link>

        <div className="rounded-2xl border border-line bg-surface p-5">
          <div className="flex items-center gap-2 text-ink-muted">
            <Package className="h-4 w-4" />
            <span className="text-xs">Pesanan</span>
          </div>
          <p className="mt-2 font-display text-lg text-ink">{ordersQuery.data?.length ?? "—"} pesanan</p>
          <p className="text-xs text-ink-muted">Sepanjang waktu</p>
        </div>
      </div>

      {wishlistQuery.data && wishlistQuery.data.length > 0 && (
        <div className="mt-10">
          <div className="flex items-center justify-between">
            <h2 className="font-display text-lg text-ink">Wishlist</h2>
            <Link href="/wishlist" className="text-sm text-accent hover:underline">
              Lihat semua
            </Link>
          </div>
          <div className="mt-3 grid grid-cols-2 gap-3 sm:grid-cols-4">
            {wishlistQuery.data.slice(0, 4).map((entry) => (
              <ProductCard key={entry.product.id} product={entry.product} />
            ))}
          </div>
        </div>
      )}

      <div className="mt-10">
        <h2 className="font-display text-lg text-ink">Riwayat Pesanan</h2>

        {ordersQuery.isLoading && <p className="mt-4 text-sm text-ink-muted">Memuat pesanan…</p>}
        {ordersQuery.data && ordersQuery.data.length === 0 && (
          <p className="mt-4 text-sm text-ink-muted">Belum ada pesanan.</p>
        )}
        {ordersQuery.data && ordersQuery.data.length > 0 && (
          <div className="mt-4 divide-y divide-line rounded-2xl border border-line bg-surface">
            {ordersQuery.data.map((order) => (
              <div key={order.id} className="flex items-center justify-between gap-4 p-4">
                <div>
                  <p className="text-sm text-ink">{order.order_number}</p>
                  <p className="text-xs text-ink-muted">
                    {new Date(order.created_at).toLocaleDateString("id-ID", {
                      day: "numeric",
                      month: "long",
                      year: "numeric",
                    })}{" "}
                    · {order.items.length} item
                  </p>
                </div>
                <div className="text-right">
                  <p className="text-sm text-ink">{formatRupiah(order.total)}</p>
                  <p className={`text-xs ${statusColor[order.status] ?? "text-ink-muted"}`}>
                    {statusLabel[order.status] ?? order.status}
                  </p>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </main>
  );
}
