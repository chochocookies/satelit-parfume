"use client";

import { useState } from "react";
import { Minus, Plus, X } from "lucide-react";
import { useMutation, useQueryClient } from "@tanstack/react-query";

import { useCart } from "@/hooks/use-cart";
import { useCartStore } from "@/stores/cart-store";
import { apiClient, type Order } from "@/lib/api-client";
import { formatRupiah } from "@/lib/format";

// Pickup-only checkout — delivery needs a real address form (React Hook
// Form + Zod, still just declared dependencies) and isn't here yet. This
// deliberately doesn't build a second, throwaway ad-hoc address form
// just to say delivery exists; pickup needs nothing but a name and a
// phone number, so it's what's real today.
export function CartPanel({ onClose }: { onClose: () => void }) {
  const { cart, isLoading, updateItem, removeItem } = useCart();
  const cartToken = useCartStore((s) => s.cartToken);
  const queryClient = useQueryClient();

  const [showCheckoutForm, setShowCheckoutForm] = useState(false);
  const [guestName, setGuestName] = useState("");
  const [guestPhone, setGuestPhone] = useState("");
  const [confirmedOrder, setConfirmedOrder] = useState<Order | null>(null);

  const checkout = useMutation({
    mutationFn: () =>
      apiClient.checkout(cartToken, {
        order_type: "pickup",
        guest_name: guestName,
        guest_phone: guestPhone,
      }),
    onSuccess: (order) => {
      setConfirmedOrder(order);
      queryClient.invalidateQueries({ queryKey: ["cart"] });
    },
  });

  if (confirmedOrder) {
    return (
      <div className="rounded-2xl border border-line bg-surface p-4 shadow-xl">
        <div className="flex items-center justify-between">
          <h3 className="font-display text-lg text-ink">Pesanan Dibuat</h3>
          <button onClick={onClose} aria-label="Tutup" className="text-ink-muted transition hover:text-ink">
            <X className="h-4 w-4" />
          </button>
        </div>
        <p className="mt-4 text-sm text-ink-muted">Nomor pesanan</p>
        <p className="text-base text-ink">{confirmedOrder.order_number}</p>
        <p className="mt-3 text-sm text-ink-muted">
          Status: <span className="text-accent">{confirmedOrder.status}</span>
        </p>
        <p className="mt-3 text-xs text-ink-muted">
          Simpan nomor ini. Pembayaran online belum tersedia (menyusul di fase berikutnya) — datang ke toko,
          tunjukkan nomor pesanan, dan bayar di kasir untuk ambil pesananmu.
        </p>
      </div>
    );
  }

  return (
    <div className="rounded-2xl border border-line bg-surface p-4 shadow-xl">
      <div className="flex items-center justify-between">
        <h3 className="font-display text-lg text-ink">Keranjang</h3>
        <button onClick={onClose} aria-label="Tutup keranjang" className="text-ink-muted transition hover:text-ink">
          <X className="h-4 w-4" />
        </button>
      </div>

      {isLoading && <p className="mt-4 text-sm text-ink-muted">Memuat keranjang…</p>}

      {!isLoading && (!cart || cart.items.length === 0) && (
        <p className="mt-4 text-sm text-ink-muted">Keranjangmu masih kosong.</p>
      )}

      {cart && cart.items.length > 0 && (
        <>
          <div className="mt-4 max-h-80 space-y-4 overflow-y-auto">
            {cart.items.map((item) => (
              <div key={item.id} className="flex items-start gap-3">
                <div className="flex-1">
                  <p className="text-sm text-ink">{item.product_name}</p>
                  <p className="text-xs text-ink-muted">{formatRupiah(item.unit_price)}</p>
                  {item.quantity > item.available_stock && (
                    <p className="mt-1 text-xs text-red-400">
                      Hanya {item.available_stock} tersedia — kurangi jumlah
                    </p>
                  )}
                  <div className="mt-2 flex items-center gap-2">
                    <button
                      onClick={() => updateItem.mutate({ itemId: item.id, quantity: Math.max(1, item.quantity - 1) })}
                      disabled={item.quantity <= 1 || updateItem.isPending}
                      aria-label="Kurangi jumlah"
                      className="flex h-6 w-6 items-center justify-center rounded-full border border-line text-ink transition disabled:opacity-40"
                    >
                      <Minus className="h-3 w-3" />
                    </button>
                    <span className="w-6 text-center text-sm text-ink">{item.quantity}</span>
                    <button
                      onClick={() => updateItem.mutate({ itemId: item.id, quantity: item.quantity + 1 })}
                      disabled={updateItem.isPending || item.quantity >= item.available_stock}
                      aria-label="Tambah jumlah"
                      className="flex h-6 w-6 items-center justify-center rounded-full border border-line text-ink transition disabled:opacity-40"
                    >
                      <Plus className="h-3 w-3" />
                    </button>
                  </div>
                </div>
                <div className="flex flex-col items-end gap-2">
                  <span className="text-sm text-ink">{formatRupiah(item.line_total)}</span>
                  <button
                    onClick={() => removeItem.mutate(item.id)}
                    className="text-xs text-ink-muted transition hover:text-red-400"
                  >
                    Hapus
                  </button>
                </div>
              </div>
            ))}
          </div>

          <div className="mt-4 flex items-center justify-between border-t border-line pt-4">
            <span className="text-sm text-ink-muted">Subtotal</span>
            <span className="text-base text-ink">{formatRupiah(cart.subtotal)}</span>
          </div>

          {!showCheckoutForm ? (
            <button
              onClick={() => setShowCheckoutForm(true)}
              className="mt-3 w-full rounded-full bg-accent px-4 py-2.5 text-sm font-medium text-background transition hover:opacity-90"
            >
              Checkout (Ambil di Toko)
            </button>
          ) : (
            <div className="mt-3 space-y-2">
              <input
                value={guestName}
                onChange={(e) => setGuestName(e.target.value)}
                placeholder="Nama"
                className="w-full rounded-full border border-line bg-background px-4 py-2 text-sm text-ink placeholder:text-ink-muted focus:border-accent focus:outline-none"
              />
              <input
                value={guestPhone}
                onChange={(e) => setGuestPhone(e.target.value)}
                placeholder="Nomor HP"
                className="w-full rounded-full border border-line bg-background px-4 py-2 text-sm text-ink placeholder:text-ink-muted focus:border-accent focus:outline-none"
              />
              <button
                onClick={() => checkout.mutate()}
                disabled={!guestName || !guestPhone || checkout.isPending}
                className="w-full rounded-full bg-accent px-4 py-2.5 text-sm font-medium text-background transition hover:opacity-90 disabled:opacity-40"
              >
                {checkout.isPending ? "Memproses…" : `Pesan Sekarang — ${formatRupiah(cart.subtotal)}`}
              </button>
              {checkout.isError && (
                <p className="text-xs text-red-400">
                  {checkout.error instanceof Error ? checkout.error.message : "Checkout gagal."}
                </p>
              )}
            </div>
          )}

          <p className="mt-3 text-xs text-ink-muted">
            Hanya ambil di toko untuk saat ini — checkout dengan pengiriman menyusul.
          </p>
        </>
      )}
    </div>
  );
}
