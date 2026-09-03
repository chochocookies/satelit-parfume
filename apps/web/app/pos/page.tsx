"use client";

import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { QRCodeSVG } from "qrcode.react";

import { apiClient, ApiError, type Branch, type Category, type Order, type Payment, type ProductListItem, type Shift } from "@/lib/api-client";
import { formatRupiah } from "@/lib/format";
import { useBranchStore } from "@/stores/branch-store";

export default function PosPage() {
  const { selectedBranch, hasHydrated } = useBranchStore();

  if (!hasHydrated) {
    return <p className="text-sm text-ink-muted">Memuat...</p>;
  }
  if (!selectedBranch) {
    return <BranchPicker />;
  }
  return <PosWorkspace branchId={selectedBranch.id} branchName={selectedBranch.name} />;
}

function BranchPicker() {
  const setBranch = useBranchStore((state) => state.setBranch);
  const branchesQuery = useQuery({ queryKey: ["branches"], queryFn: () => apiClient.listBranches() });

  return (
    <div className="mx-auto max-w-sm">
      <h1 className="font-display text-xl text-ink">Pilih Cabang</h1>
      <p className="mt-1 text-sm text-ink-muted">Pilih cabang tempat Anda bertugas hari ini.</p>
      <div className="mt-4 flex flex-col gap-2">
        {branchesQuery.isLoading && <p className="text-sm text-ink-muted">Memuat cabang...</p>}
        {branchesQuery.data?.map((b: Branch) => (
          <button
            key={b.id}
            onClick={() => setBranch(b)}
            className="rounded-2xl border border-line bg-surface p-4 text-left transition hover:border-accent"
          >
            <p className="text-ink">{b.name}</p>
            {b.city && <p className="text-xs text-ink-muted">{b.city}</p>}
          </button>
        ))}
      </div>
    </div>
  );
}

function PosWorkspace({ branchId, branchName }: { branchId: string; branchName: string }) {
  const shiftQuery = useQuery({
    queryKey: ["pos", "current-shift", branchId],
    queryFn: () => apiClient.posCurrentShift(branchId),
  });

  if (shiftQuery.isLoading) {
    return <p className="text-sm text-ink-muted">Memuat status shift...</p>;
  }

  if (!shiftQuery.data) {
    return <OpenShiftForm branchId={branchId} branchName={branchName} />;
  }

  return <SaleScreen branchId={branchId} branchName={branchName} shift={shiftQuery.data} />;
}

function OpenShiftForm({ branchId, branchName }: { branchId: string; branchName: string }) {
  const queryClient = useQueryClient();
  const [openingBalance, setOpeningBalance] = useState("");
  const [notes, setNotes] = useState("");
  const [error, setError] = useState<string | null>(null);

  const openMutation = useMutation({
    mutationFn: () => apiClient.posOpenShift(branchId, { opening_balance: Number(openingBalance || 0), notes }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["pos", "current-shift", branchId] }),
    onError: (err) => setError(err instanceof ApiError ? err.message : "Gagal membuka shift."),
  });

  return (
    <div className="mx-auto max-w-sm">
      <h1 className="font-display text-xl text-ink">Buka Shift — {branchName}</h1>
      <p className="mt-1 text-sm text-ink-muted">Masukkan jumlah kas awal di laci sebelum mulai melayani transaksi.</p>

      <form
        onSubmit={(e) => {
          e.preventDefault();
          setError(null);
          openMutation.mutate();
        }}
        className="mt-4 flex flex-col gap-3"
      >
        <div className="flex flex-col gap-1.5">
          <label className="text-sm text-ink-muted">Kas Awal (Rp)</label>
          <input
            required
            type="number"
            min={0}
            value={openingBalance}
            onChange={(e) => setOpeningBalance(e.target.value)}
            className="rounded-full border border-line bg-background px-4 py-2 text-sm text-ink placeholder:text-ink-muted focus:border-accent focus:outline-none"
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <label className="text-sm text-ink-muted">Catatan (opsional)</label>
          <input
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
            className="rounded-full border border-line bg-background px-4 py-2 text-sm text-ink placeholder:text-ink-muted focus:border-accent focus:outline-none"
          />
        </div>
        {error && <p className="text-sm text-red-400">{error}</p>}
        <button
          type="submit"
          disabled={openMutation.isPending}
          className="mt-2 rounded-full bg-accent px-4 py-2.5 text-sm font-medium text-background transition hover:opacity-90 disabled:opacity-40"
        >
          {openMutation.isPending ? "Membuka..." : "Buka Shift"}
        </button>
      </form>
    </div>
  );
}

type SaleStage =
  | { name: "cart" }
  | { name: "qris-wait"; order: Order; payment: Payment }
  | { name: "receipt"; order: Order }
  | { name: "close" }
  | { name: "close-summary"; shift: Shift };

function SaleScreen({ branchId, branchName, shift }: { branchId: string; branchName: string; shift: Shift }) {
  const queryClient = useQueryClient();
  const [cartToken, setCartToken] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [searchSubmitted, setSearchSubmitted] = useState("");
  // "" (Semua) plus every category slug from GET /api/v1/categories —
  // same slug convention ShopPageClient already filters by, so a
  // cashier and a customer never see the catalog grouped two different
  // ways.
  const [activeCategory, setActiveCategory] = useState("");
  const [stage, setStage] = useState<SaleStage>({ name: "cart" });
  const [error, setError] = useState<string | null>(null);

  const cartQuery = useQuery({
    queryKey: ["pos", "cart", cartToken],
    queryFn: () => apiClient.getCart(cartToken),
    enabled: cartToken !== null,
  });

  const categoriesQuery = useQuery({ queryKey: ["categories"], queryFn: apiClient.listCategories });

  // Browsable by default — no longer gated behind typing a search term
  // first. A cashier ringing up a walk-in needs to tap through what's
  // in front of them, the same way the customer-facing shop already
  // works; search narrows it further when they DO know what they want.
  const productsQuery = useQuery({
    queryKey: ["pos", "products", branchId, searchSubmitted, activeCategory],
    queryFn: () =>
      apiClient.listProducts({
        search: searchSubmitted || undefined,
        category: activeCategory || undefined,
        branch: branchId,
        limit: 24,
      }),
  });

  function syncCartToken(token?: string) {
    if (token && token !== cartToken) {
      setCartToken(token);
    }
  }

  const addItemMutation = useMutation({
    mutationFn: async (item: ProductListItem) => {
      const detail = await apiClient.getProduct(item.slug, branchId);
      const variant = detail.variants[0];
      if (!variant) throw new Error("Produk ini belum punya varian.");
      return apiClient.addCartItem(cartToken, { product_variant_id: variant.id, branch_id: branchId, quantity: 1 });
    },
    onSuccess: (cart) => {
      syncCartToken(cart.session_token);
      queryClient.setQueryData(["pos", "cart", cart.session_token ?? cartToken], cart);
      setError(null);
    },
    onError: (err) => setError(err instanceof ApiError || err instanceof Error ? err.message : "Gagal menambah item."),
  });

  const updateItemMutation = useMutation({
    mutationFn: ({ itemId, quantity }: { itemId: string; quantity: number }) =>
      apiClient.updateCartItem(cartToken, itemId, quantity),
    onSuccess: (cart) => queryClient.setQueryData(["pos", "cart", cartToken], cart),
  });

  const removeItemMutation = useMutation({
    mutationFn: (itemId: string) => apiClient.removeCartItem(cartToken, itemId),
    onSuccess: (cart) => queryClient.setQueryData(["pos", "cart", cartToken], cart),
  });

  const checkoutMutation = useMutation({
    mutationFn: (method: "cash" | "qris") =>
      apiClient.posCheckout(branchId, cartToken, { order_type: "pickup", guest_name: "", guest_phone: "" }).then(async (order) => {
        if (method === "cash") {
          return { order: await apiClient.posConfirmCash(branchId, order.id), method };
        }
        return { order, method };
      }),
    onSuccess: async ({ order, method }) => {
      setError(null);
      if (method === "cash") {
        setStage({ name: "receipt", order });
        setCartToken(null);
        return;
      }
      try {
        const payment = await apiClient.posPay(branchId, order.id, "SP");
        setStage({ name: "qris-wait", order, payment });
        setCartToken(null);
      } catch (err) {
        setError(err instanceof ApiError ? err.message : "Gagal membuat pembayaran QRIS.");
      }
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Gagal membuat transaksi."),
  });

  if (stage.name === "qris-wait") {
    return (
      <QrisWaitScreen
        branchId={branchId}
        order={stage.order}
        payment={stage.payment}
        onPaid={(order) => setStage({ name: "receipt", order })}
        onCancel={() => setStage({ name: "cart" })}
      />
    );
  }

  if (stage.name === "receipt") {
    return (
      <ReceiptScreen
        order={stage.order}
        onNewSale={() => {
          setStage({ name: "cart" });
          setSearch("");
          setSearchSubmitted("");
        }}
      />
    );
  }

  if (stage.name === "close") {
    return (
      <CloseShiftForm
        branchId={branchId}
        shift={shift}
        onClosed={(closed) => setStage({ name: "close-summary", shift: closed })}
        onCancel={() => setStage({ name: "cart" })}
      />
    );
  }

  if (stage.name === "close-summary") {
    return <ShiftSummary shift={stage.shift} />;
  }

  const cart = cartQuery.data;

  return (
    <div className="grid gap-4 lg:grid-cols-[1fr_360px]">
      <div className="flex flex-col gap-4">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <h1 className="font-display text-xl text-ink">Kasir — {branchName}</h1>
          <button onClick={() => setStage({ name: "close" })} className="rounded-full border border-line px-4 py-2 text-sm text-ink">
            Tutup Shift
          </button>
        </div>

        <form
          onSubmit={(e) => {
            e.preventDefault();
            setSearchSubmitted(search.trim());
          }}
          className="flex gap-2"
        >
          <input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Cari nama produk atau SKU..."
            className="flex-1 rounded-full border border-line bg-background px-4 py-3 text-base text-ink placeholder:text-ink-muted focus:border-accent focus:outline-none"
          />
          <button
            type="submit"
            className="rounded-full border border-line px-5 py-3 text-sm font-medium text-ink active:scale-95"
          >
            Cari
          </button>
        </form>

        {/* Category pills — "Semua" plus every real category, so a
            cashier browses by tapping instead of having to know what to
            type. Horizontally scrollable rather than wrapping: on a
            tablet-width touchscreen a scrolling row keeps every pill at
            a consistent, thumb-sized target instead of shrinking to fit. */}
        <div className="-mx-1 flex gap-2 overflow-x-auto px-1 pb-1">
          <button
            onClick={() => setActiveCategory("")}
            className={`shrink-0 rounded-full border px-4 py-2.5 text-sm font-medium transition active:scale-95 ${
              activeCategory === ""
                ? "border-accent bg-accent text-background"
                : "border-line bg-surface text-ink-muted hover:border-accent/50"
            }`}
          >
            Semua
          </button>
          {categoriesQuery.data?.map((c: Category) => (
            <button
              key={c.id}
              onClick={() => setActiveCategory(c.slug)}
              className={`shrink-0 rounded-full border px-4 py-2.5 text-sm font-medium transition active:scale-95 ${
                activeCategory === c.slug
                  ? "border-accent bg-accent text-background"
                  : "border-line bg-surface text-ink-muted hover:border-accent/50"
              }`}
            >
              {c.name}
            </button>
          ))}
        </div>

        {productsQuery.isLoading && <p className="text-sm text-ink-muted">Memuat produk...</p>}
        {productsQuery.data && (
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-4">
            {productsQuery.data.items.map((item) => {
              const outOfStock = (item.branch_stock ?? 0) <= 0;
              return (
                <button
                  key={item.id}
                  onClick={() => addItemMutation.mutate(item)}
                  disabled={addItemMutation.isPending || outOfStock}
                  className="flex flex-col overflow-hidden rounded-2xl border border-line bg-surface text-left transition active:scale-[0.97] disabled:cursor-not-allowed disabled:opacity-40"
                >
                  <div className="flex aspect-square items-center justify-center overflow-hidden bg-background">
                    {item.primary_image_url ? (
                      // eslint-disable-next-line @next/next/no-img-element
                      <img src={item.primary_image_url} alt={item.name} className="h-full w-full object-cover" />
                    ) : (
                      <span className="text-2xl text-ink-muted">🧴</span>
                    )}
                  </div>
                  <div className="flex flex-1 flex-col gap-0.5 p-3">
                    <p className="line-clamp-2 text-sm text-ink">{item.name}</p>
                    <p className="text-xs text-ink-muted">{item.brand?.name ?? "-"}</p>
                    <div className="mt-1.5 flex items-center justify-between">
                      <span className="text-sm font-medium text-accent">{formatRupiah(item.price_from)}</span>
                      <span className={`text-xs ${outOfStock ? "text-red-400" : "text-ink-muted"}`}>
                        {outOfStock ? "Habis" : `Stok ${item.branch_stock ?? "-"}`}
                      </span>
                    </div>
                  </div>
                </button>
              );
            })}
            {productsQuery.data.items.length === 0 && (
              <p className="col-span-full py-6 text-center text-sm text-ink-muted">Tidak ada produk ditemukan.</p>
            )}
          </div>
        )}

        {error && <p className="text-sm text-red-400">{error}</p>}
      </div>

      <div className="flex flex-col gap-3 rounded-2xl border border-line bg-surface p-4">
        <h2 className="text-sm font-medium text-ink">Keranjang</h2>

        <div className="flex flex-col divide-y divide-line">
          {cart?.items.map((line) => (
            <div key={line.id} className="flex items-center justify-between gap-2 py-2.5 text-sm">
              <div className="min-w-0 flex-1">
                <p className="truncate text-ink">{line.product_name}</p>
                <p className="text-xs text-ink-muted">{formatRupiah(line.unit_price)}</p>
              </div>
              <div className="flex items-center gap-1.5">
                <button
                  onClick={() => updateItemMutation.mutate({ itemId: line.id, quantity: Math.max(0, line.quantity - 1) })}
                  className="flex h-9 w-9 items-center justify-center rounded-full border border-line text-base text-ink active:scale-90"
                >
                  −
                </button>
                <span className="w-6 text-center text-ink">{line.quantity}</span>
                <button
                  onClick={() => updateItemMutation.mutate({ itemId: line.id, quantity: line.quantity + 1 })}
                  className="flex h-9 w-9 items-center justify-center rounded-full border border-line text-base text-ink active:scale-90"
                >
                  +
                </button>
              </div>
              <button onClick={() => removeItemMutation.mutate(line.id)} className="text-xs text-red-400">
                Hapus
              </button>
            </div>
          ))}
          {(!cart || cart.items.length === 0) && <p className="py-3 text-sm text-ink-muted">Keranjang kosong.</p>}
        </div>

        <div className="flex items-center justify-between border-t border-line pt-3 text-sm font-medium">
          <span className="text-ink">Total</span>
          <span className="text-ink">{formatRupiah(cart?.subtotal ?? 0)}</span>
        </div>

        <div className="mt-2 flex flex-col gap-2">
          <button
            disabled={!cart || cart.items.length === 0 || checkoutMutation.isPending}
            onClick={() => checkoutMutation.mutate("cash")}
            className="rounded-full bg-accent px-4 py-3.5 text-base font-medium text-background transition hover:opacity-90 active:scale-[0.98] disabled:opacity-40"
          >
            {checkoutMutation.isPending ? "Memproses..." : "Bayar Tunai"}
          </button>
          <button
            disabled={!cart || cart.items.length === 0 || checkoutMutation.isPending}
            onClick={() => checkoutMutation.mutate("qris")}
            className="rounded-full border border-accent px-4 py-3.5 text-base font-medium text-accent transition hover:bg-accent/10 active:scale-[0.98] disabled:opacity-40"
          >
            Bayar QRIS
          </button>
        </div>
      </div>
    </div>
  );
}

function QrisWaitScreen({
  branchId,
  order,
  payment,
  onPaid,
  onCancel,
}: {
  branchId: string;
  order: Order;
  payment: Payment;
  onPaid: (order: Order) => void;
  onCancel: () => void;
}) {
  const paymentQuery = useQuery({
    queryKey: ["pos", "payment-poll", order.id],
    queryFn: () => apiClient.posGetOrder(branchId, order.id),
    refetchInterval: (query) => (query.state.data?.status === "PAID" ? false : 3000),
  });

  // Polling stops itself once status is PAID (refetchInterval above), so
  // this only ever fires once — the single moment paymentQuery.data's
  // reference changes from "still pending" to the fresh PAID order.
  // Deliberately a useEffect rather than calling onPaid directly in the
  // render body: doing it there would update SaleScreen's state while
  // this component is still rendering.
  useEffect(() => {
    if (paymentQuery.data?.status === "PAID") {
      onPaid(paymentQuery.data);
    }
  }, [paymentQuery.data, onPaid]);

  return (
    <div className="mx-auto flex max-w-sm flex-col items-center gap-4 text-center">
      <h1 className="font-display text-xl text-ink">Pindai untuk Bayar</h1>
      <p className="text-sm text-ink-muted">
        Total <span className="text-ink">{formatRupiah(order.total)}</span> — {order.order_number}
      </p>
      <div className="rounded-2xl border border-line bg-white p-4">
        {/* payment.qr_string is Duitku's actual QRIS payload — what a
            real payment app needs to scan-and-pay. Falling back to the
            order number would render *something*, but it's not a valid
            QRIS code and a cashier scanning it against a real wallet
            app would only ever get a "can't read this" error. */}
        <QRCodeSVG value={payment.qr_string || order.order_number} size={220} />
      </div>
      <p className="text-sm text-ink-muted">Menunggu konfirmasi pembayaran...</p>
      <button onClick={onCancel} className="rounded-full border border-line px-4 py-2 text-sm text-ink">
        Batal
      </button>
    </div>
  );
}

function ReceiptScreen({ order, onNewSale }: { order: Order; onNewSale: () => void }) {
  return (
    <div className="mx-auto flex max-w-sm flex-col gap-4">
      <div className="rounded-2xl border border-line bg-surface p-5">
        <h1 className="font-display text-lg text-ink">Struk Transaksi</h1>
        <p className="text-xs text-ink-muted">{order.order_number}</p>

        <div className="mt-4 flex flex-col divide-y divide-line text-sm">
          {order.items.map((item) => (
            <div key={item.id} className="flex items-center justify-between py-2">
              <div>
                <p className="text-ink">{item.product_name}</p>
                <p className="text-xs text-ink-muted">
                  {item.quantity} × {formatRupiah(item.unit_price)}
                </p>
              </div>
              <span className="text-ink">{formatRupiah(item.subtotal)}</span>
            </div>
          ))}
        </div>

        <div className="mt-3 flex items-center justify-between border-t border-line pt-3 text-sm font-medium">
          <span className="text-ink">Total</span>
          <span className="text-ink">{formatRupiah(order.total)}</span>
        </div>
        <p className="mt-1 text-xs text-ink-muted">
          Dibayar via {order.payment_method === "cash" ? "Tunai" : "QRIS"}
        </p>
      </div>

      <div className="flex gap-3 print:hidden">
        <button onClick={() => window.print()} className="flex-1 rounded-full border border-line px-4 py-2 text-sm text-ink">
          Cetak
        </button>
        <button
          onClick={onNewSale}
          className="flex-1 rounded-full bg-accent px-4 py-2.5 text-sm font-medium text-background transition hover:opacity-90"
        >
          Transaksi Baru
        </button>
      </div>
    </div>
  );
}

function CloseShiftForm({
  branchId,
  shift,
  onClosed,
  onCancel,
}: {
  branchId: string;
  shift: Shift;
  onClosed: (shift: Shift) => void;
  onCancel: () => void;
}) {
  const queryClient = useQueryClient();
  const [closingBalance, setClosingBalance] = useState("");
  const [notes, setNotes] = useState("");
  const [error, setError] = useState<string | null>(null);

  const closeMutation = useMutation({
    mutationFn: () => apiClient.posCloseShift(branchId, shift.id, { closing_balance: Number(closingBalance || 0), notes }),
    onSuccess: (closed) => {
      queryClient.invalidateQueries({ queryKey: ["pos", "current-shift", branchId] });
      onClosed(closed);
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Gagal menutup shift."),
  });

  return (
    <div className="mx-auto max-w-sm">
      <h1 className="font-display text-xl text-ink">Tutup Shift</h1>
      <p className="mt-1 text-sm text-ink-muted">Hitung kas fisik di laci dan masukkan jumlahnya di bawah ini.</p>

      <form
        onSubmit={(e) => {
          e.preventDefault();
          setError(null);
          closeMutation.mutate();
        }}
        className="mt-4 flex flex-col gap-3"
      >
        <div className="flex flex-col gap-1.5">
          <label className="text-sm text-ink-muted">Kas Akhir Dihitung (Rp)</label>
          <input
            required
            type="number"
            min={0}
            value={closingBalance}
            onChange={(e) => setClosingBalance(e.target.value)}
            className="rounded-full border border-line bg-background px-4 py-2 text-sm text-ink placeholder:text-ink-muted focus:border-accent focus:outline-none"
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <label className="text-sm text-ink-muted">Catatan (opsional)</label>
          <input
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
            className="rounded-full border border-line bg-background px-4 py-2 text-sm text-ink placeholder:text-ink-muted focus:border-accent focus:outline-none"
          />
        </div>
        {error && <p className="text-sm text-red-400">{error}</p>}
        <div className="mt-2 flex gap-3">
          <button type="button" onClick={onCancel} className="flex-1 rounded-full border border-line px-4 py-2 text-sm text-ink">
            Batal
          </button>
          <button
            type="submit"
            disabled={closeMutation.isPending}
            className="flex-1 rounded-full bg-accent px-4 py-2.5 text-sm font-medium text-background transition hover:opacity-90 disabled:opacity-40"
          >
            {closeMutation.isPending ? "Menutup..." : "Tutup Shift"}
          </button>
        </div>
      </form>
    </div>
  );
}

function ShiftSummary({ shift }: { shift: Shift }) {
  const discrepancy = shift.discrepancy ?? 0;

  return (
    <div className="mx-auto flex max-w-sm flex-col gap-4">
      <div className="rounded-2xl border border-line bg-surface p-5">
        <h1 className="font-display text-lg text-ink">Ringkasan Shift</h1>
        <dl className="mt-4 flex flex-col gap-2 text-sm">
          <Row label="Kas Awal" value={formatRupiah(shift.opening_balance)} />
          <Row label="Kas Diharapkan" value={formatRupiah(shift.expected_balance ?? 0)} />
          <Row label="Kas Dihitung" value={formatRupiah(shift.closing_balance ?? 0)} />
          <Row
            label="Selisih"
            value={`${discrepancy > 0 ? "+" : ""}${formatRupiah(discrepancy)}`}
            emphasis={discrepancy !== 0}
          />
        </dl>
      </div>
      <button onClick={() => window.print()} className="rounded-full border border-line px-4 py-2 text-sm text-ink print:hidden">
        Cetak
      </button>
      <p className="text-center text-xs text-ink-muted">Muat ulang halaman untuk membuka shift baru.</p>
    </div>
  );
}

function Row({ label, value, emphasis }: { label: string; value: string; emphasis?: boolean }) {
  return (
    <div className="flex justify-between gap-4">
      <dt className="text-ink-muted">{label}</dt>
      <dd className={emphasis ? "font-medium text-red-400" : "text-ink"}>{value}</dd>
    </div>
  );
}
