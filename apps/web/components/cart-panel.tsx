"use client";

import { useState } from "react";
import Link from "next/link";
import { Minus, Plus, X } from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { QRCodeSVG } from "qrcode.react";

import { useCart } from "@/hooks/use-cart";
import { useCartStore } from "@/stores/cart-store";
import { useCustomerAuthStore } from "@/stores/customer-auth-store";
import { apiClient, ApiError, type Order, type Payment } from "@/lib/api-client";
import { formatRupiah } from "@/lib/format";

// Duitku's QRIS method code — "SP" throughout this project (see
// internal/payments' Phase 8 note on why the verified codes are kept
// separate from guessed ones); the same constant app/pos/page.tsx uses.
const QRIS_METHOD = "SP";

type PaymentChoice = "counter" | "online";

// Pickup-only checkout — delivery needs a real address form (React Hook
// Form + Zod, still just declared dependencies) and isn't here yet. This
// deliberately doesn't build a second, throwaway ad-hoc address form
// just to say delivery exists; pickup needs nothing but a name and a
// phone number, so it's what's real today.
//
// Payment timing is now a real choice, not just pickup-and-pay-later:
// a logged-in customer can pay online right now via Duitku QRIS (Phase
// 8's backend, previously with no frontend — see the root README's
// "What Phase 8 adds"), or stick with the original pay-at-counter flow.
// Guests keep the original pay-at-counter-only path — POST
// /orders/:id/pay requires an authenticated customer (Duitku needs an
// email, and guest checkout was never extended to collect one), so
// there's no honest way to offer "pay online now" without asking a
// guest to log in first.
export function CartPanel({ onClose }: { onClose: () => void }) {
  const { cart, isLoading, updateItem, removeItem } = useCart();
  const cartToken = useCartStore((s) => s.cartToken);
  const customerAccessToken = useCustomerAuthStore((s) => s.accessToken);
  const customer = useCustomerAuthStore((s) => s.customer);
  const customerHydrated = useCustomerAuthStore((s) => s.hasHydrated);
  const queryClient = useQueryClient();

  const isCustomer = customerHydrated && !!customerAccessToken;

  const [showCheckoutForm, setShowCheckoutForm] = useState(false);
  const [guestName, setGuestName] = useState("");
  const [guestPhone, setGuestPhone] = useState("");
  const [guestEmail, setGuestEmail] = useState("");
  const [paymentChoice, setPaymentChoice] = useState<PaymentChoice>("counter");
  const [confirmedOrder, setConfirmedOrder] = useState<Order | null>(null);
  const [payment, setPayment] = useState<Payment | null>(null);
  const [payError, setPayError] = useState<string | null>(null);

  function openCheckoutForm() {
    // Prefill from the logged-in customer's own account — a nicety, not
    // a requirement (both fields stay fully editable): saves retyping a
    // name/email that's already on file, without pretending to know a
    // phone number AdminSubject doesn't carry.
    if (isCustomer && customer) {
      setGuestName((prev) => prev || customer.name);
      setGuestEmail((prev) => prev || customer.email);
    }
    setShowCheckoutForm(true);
  }

  const checkout = useMutation({
    mutationFn: async () => {
      const order = await apiClient.checkout(
        cartToken,
        {
          order_type: "pickup",
          guest_name: guestName,
          guest_phone: guestPhone,
          guest_email: paymentChoice === "online" ? guestEmail : undefined,
        },
        isCustomer ? customerAccessToken : undefined,
      );

      if (paymentChoice === "online") {
        try {
          const createdPayment = await apiClient.payOrder(order.id, QRIS_METHOD);
          return { order, payment: createdPayment, payError: null as string | null };
        } catch (err) {
          // The order itself is real and already reserved — a payment
          // that failed to start (Duitku unreachable, misconfigured
          // sandbox keys, etc.) shouldn't make it look like checkout
          // itself failed. Fall through to the same "come pay at the
          // counter" outcome pay-at-counter would have given.
          const message = err instanceof ApiError ? err.message : "Gagal membuat pembayaran online.";
          return { order, payment: null as Payment | null, payError: message };
        }
      }
      return { order, payment: null as Payment | null, payError: null as string | null };
    },
    onSuccess: ({ order, payment: createdPayment, payError: creationError }) => {
      setConfirmedOrder(order);
      setPayment(createdPayment);
      setPayError(creationError);
      queryClient.invalidateQueries({ queryKey: ["cart"] });
    },
  });

  if (confirmedOrder) {
    return (
      <OrderConfirmedPanel
        order={confirmedOrder}
        initialPayment={payment}
        payError={payError}
        onClose={onClose}
      />
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
              onClick={openCheckoutForm}
              className="mt-3 w-full rounded-full bg-accent px-4 py-2.5 text-sm font-medium text-background transition hover:opacity-90"
            >
              Checkout (Ambil di Toko)
            </button>
          ) : (
            <div className="mt-3 space-y-3">
              <div className="space-y-2">
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
              </div>

              <div className="space-y-1.5">
                <p className="text-xs text-ink-muted">Kapan mau bayar?</p>
                <div className="grid grid-cols-2 gap-2">
                  <button
                    type="button"
                    onClick={() => setPaymentChoice("counter")}
                    className={`rounded-2xl border px-3 py-2.5 text-sm transition ${
                      paymentChoice === "counter"
                        ? "border-accent bg-accent/10 text-accent"
                        : "border-line text-ink-muted hover:border-accent/50"
                    }`}
                  >
                    Bayar di Toko
                  </button>
                  <button
                    type="button"
                    onClick={() => isCustomer && setPaymentChoice("online")}
                    disabled={!isCustomer}
                    title={isCustomer ? undefined : "Masuk untuk bayar online"}
                    className={`rounded-2xl border px-3 py-2.5 text-sm transition disabled:cursor-not-allowed disabled:opacity-40 ${
                      paymentChoice === "online"
                        ? "border-accent bg-accent/10 text-accent"
                        : "border-line text-ink-muted hover:border-accent/50"
                    }`}
                  >
                    Bayar Online (QRIS)
                  </button>
                </div>
                {!isCustomer && (
                  <p className="text-xs text-ink-muted">
                    <Link href="/login" className="text-accent hover:underline">
                      Masuk
                    </Link>{" "}
                    dulu untuk bayar online sekarang lewat QRIS — bayar di toko tetap bisa tanpa akun.
                  </p>
                )}
              </div>

              {paymentChoice === "online" && (
                <input
                  type="email"
                  value={guestEmail}
                  onChange={(e) => setGuestEmail(e.target.value)}
                  placeholder="Email (untuk bukti pembayaran Duitku)"
                  className="w-full rounded-full border border-line bg-background px-4 py-2 text-sm text-ink placeholder:text-ink-muted focus:border-accent focus:outline-none"
                />
              )}

              <button
                onClick={() => checkout.mutate()}
                disabled={
                  !guestName ||
                  !guestPhone ||
                  (paymentChoice === "online" && !guestEmail) ||
                  checkout.isPending
                }
                className="w-full rounded-full bg-accent px-4 py-2.5 text-sm font-medium text-background transition hover:opacity-90 disabled:opacity-40"
              >
                {checkout.isPending
                  ? "Memproses…"
                  : paymentChoice === "online"
                    ? `Bayar Sekarang — ${formatRupiah(cart.subtotal)}`
                    : `Pesan Sekarang — ${formatRupiah(cart.subtotal)}`}
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

// OrderConfirmedPanel covers all three post-checkout outcomes: pay at
// counter (shows a QR of the order number itself — scannable by staff,
// with the number also printed above for anyone reading it out instead),
// paying online now (polls for the webhook the same way
// app/pos/page.tsx's QrisWaitScreen does), and the
// order-succeeded-but-payment-creation-failed edge case, which folds
// into the same pay-at-counter view plus one extra line explaining why.
function OrderConfirmedPanel({
  order,
  initialPayment,
  payError,
  onClose,
}: {
  order: Order;
  initialPayment: Payment | null;
  payError: string | null;
  onClose: () => void;
}) {
  // refetchInterval returning false only stops the *next* scheduled
  // fetch — it doesn't clear the data already in the cache. So once a
  // poll comes back PAID, paymentPoll.data already holds that result
  // for good; no separate "remember it" state or effect is needed, and
  // deriving currentPayment straight from render avoids the
  // setState-in-effect cascade that pattern would otherwise cause.
  const paymentPoll = useQuery({
    queryKey: ["order-payment-poll", order.id],
    queryFn: () => apiClient.getOrderPayment(order.id),
    enabled: !!initialPayment && initialPayment.status !== "PAID",
    refetchInterval: (query) => (query.state.data?.status === "PAID" ? false : 3000),
  });

  const currentPayment = paymentPoll.data ?? initialPayment;

  return (
    <div className="rounded-2xl border border-line bg-surface p-4 shadow-xl">
      <div className="flex items-center justify-between">
        <h3 className="font-display text-lg text-ink">Pesanan Dibuat</h3>
        <button onClick={onClose} aria-label="Tutup" className="text-ink-muted transition hover:text-ink">
          <X className="h-4 w-4" />
        </button>
      </div>
      <p className="mt-4 text-sm text-ink-muted">Nomor pesanan</p>
      <p className="text-base text-ink">{order.order_number}</p>

      {currentPayment && currentPayment.status !== "PAID" && (
        <>
          <p className="mt-4 text-sm text-ink-muted">
            Total <span className="text-ink">{formatRupiah(order.total)}</span>
          </p>
          <div className="mt-3 flex justify-center rounded-2xl border border-line bg-white p-4">
            <QRCodeSVG value={currentPayment.qr_string || order.order_number} size={200} />
          </div>
          <p className="mt-3 text-center text-xs text-ink-muted">Pindai dengan aplikasi QRIS apa pun untuk membayar.</p>
          <p className="mt-1 text-center text-xs text-ink-muted">Menunggu konfirmasi pembayaran…</p>
        </>
      )}

      {currentPayment && currentPayment.status === "PAID" && (
        <>
          <p className="mt-3 text-sm text-emerald-400">✓ Pembayaran diterima</p>
          <p className="mt-2 text-xs text-ink-muted">
            Pesananmu sudah lunas. Tunjukkan nomor pesanan ini saat mengambil di toko.
          </p>
        </>
      )}

      {!currentPayment && (
        <>
          {payError && <p className="mt-3 text-xs text-red-400">Pembayaran online gagal dibuat: {payError}</p>}
          {/* Same QR pattern as the online-payment branch above, but
              encoding the order number rather than a qr_string — there's
              no Duitku payment behind a pay-at-counter order to scan,
              just the order itself. Staff can scan it to pull the order
              up instantly, and the number stays visible above either
              way, so this only adds an option rather than replacing the
              "just read it out" fallback that already worked fine. */}
          <div className="mt-3 flex justify-center rounded-2xl border border-line bg-white p-4">
            <QRCodeSVG value={order.order_number} size={200} />
          </div>
          <p className="mt-3 text-center text-xs text-ink-muted">
            Tunjukkan QR ini untuk dipindai kasir, atau sebutkan nomor pesanan di atas — lalu bayar di kasir untuk
            ambil pesananmu.
          </p>
        </>
      )}
    </div>
  );
}
