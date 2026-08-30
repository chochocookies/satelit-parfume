"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowRight, CheckCircle2, Plus, XCircle } from "lucide-react";

import { apiClient, ApiError, type Branch, type ProductListItem, type StockTransfer, type TransferItem } from "@/lib/api-client";
import { useBranchStore } from "@/stores/branch-store";
import { BranchPicker } from "@/components/staff/branch-picker";

const inputClass =
  "rounded-full border border-line bg-background px-4 py-2 text-sm text-ink placeholder:text-ink-muted focus:border-accent focus:outline-none";

const STATUS_LABELS: Record<string, string> = { pending: "Menunggu", completed: "Selesai", cancelled: "Dibatalkan" };
const STATUS_STYLES: Record<string, string> = {
  pending: "bg-amber-400/10 text-amber-300 border-amber-400/30",
  completed: "bg-emerald-400/10 text-emerald-300 border-emerald-400/30",
  cancelled: "bg-ink-muted/10 text-ink-muted border-line",
};

function TransferStatusBadge({ status }: { status: string }) {
  return (
    <span className={`inline-flex items-center rounded-full border px-3 py-1 text-xs font-medium ${STATUS_STYLES[status]}`}>
      {STATUS_LABELS[status] ?? status}
    </span>
  );
}

export default function TransfersPage() {
  const { selectedBranch, hasHydrated } = useBranchStore();

  if (!hasHydrated) {
    return <p className="text-sm text-ink-muted">Memuat...</p>;
  }
  if (!selectedBranch) {
    return <BranchPicker title="Pilih Cabang" subtitle="Pilih cabang untuk mengelola transfer stok." />;
  }
  return <TransfersWorkspace branchId={selectedBranch.id} branchName={selectedBranch.name} />;
}

function TransfersWorkspace({ branchId, branchName }: { branchId: string; branchName: string }) {
  const queryClient = useQueryClient();
  const [showForm, setShowForm] = useState(false);

  const transfersQuery = useQuery({
    queryKey: ["admin", "transfers", branchId],
    queryFn: () => apiClient.adminListTransfers(branchId),
  });

  const branchesQuery = useQuery({ queryKey: ["branches"], queryFn: () => apiClient.listBranches() });
  const branchName2 = (id: string) => branchesQuery.data?.find((b: Branch) => b.id === id)?.name ?? id;

  function invalidate() {
    queryClient.invalidateQueries({ queryKey: ["admin", "transfers", branchId] });
    queryClient.invalidateQueries({ queryKey: ["admin", "inventory", branchId] });
  }

  const completeMutation = useMutation({
    mutationFn: (transferId: string) => apiClient.adminCompleteTransfer(branchId, transferId),
    onSuccess: invalidate,
  });

  const cancelMutation = useMutation({
    mutationFn: (transferId: string) => apiClient.adminCancelTransfer(branchId, transferId),
    onSuccess: invalidate,
  });

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="font-display text-2xl text-ink">Transfer Stok — {branchName}</h1>
          <p className="mt-1 text-sm text-ink-muted">Transfer keluar dan masuk yang melibatkan cabang ini.</p>
        </div>
        <button
          onClick={() => setShowForm(true)}
          className="flex items-center gap-2 rounded-full bg-accent px-4 py-2.5 text-sm font-medium text-background transition hover:opacity-90"
        >
          <Plus className="h-4 w-4" />
          Buat Transfer
        </button>
      </div>

      {transfersQuery.isLoading && <p className="text-sm text-ink-muted">Memuat transfer...</p>}

      {transfersQuery.data && (
        <div className="flex flex-col gap-3">
          {transfersQuery.data.map((t: StockTransfer) => {
            const isOutgoing = t.from_branch_id === branchId;
            return (
              <div key={t.id} className="rounded-2xl border border-line bg-surface p-4">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <p className="flex items-center gap-1.5 text-sm text-ink">
                    <ArrowRight className={`h-3.5 w-3.5 text-ink-muted ${isOutgoing ? "" : "rotate-180"}`} aria-hidden="true" />
                    {isOutgoing ? "Ke" : "Dari"} <span className="font-medium">{branchName2(isOutgoing ? t.to_branch_id : t.from_branch_id)}</span>
                  </p>
                  <TransferStatusBadge status={t.status} />
                </div>
                <div className="mt-2 flex flex-col gap-1 text-sm text-ink-muted">
                  {t.items.map((item: TransferItem) => (
                    <p key={item.product_variant_id}>
                      {item.product_name} ({item.variant_name}) × {item.quantity}
                    </p>
                  ))}
                </div>
                {t.status === "pending" && (
                  <div className="mt-3 flex gap-3">
                    {!isOutgoing && (
                      <button
                        onClick={() => completeMutation.mutate(t.id)}
                        disabled={completeMutation.isPending}
                        className="flex items-center gap-1.5 rounded-full bg-accent px-4 py-2 text-sm font-medium text-background transition hover:opacity-90 disabled:opacity-40"
                      >
                        <CheckCircle2 className="h-4 w-4" />
                        Terima Transfer
                      </button>
                    )}
                    {isOutgoing && (
                      <button
                        onClick={() => cancelMutation.mutate(t.id)}
                        disabled={cancelMutation.isPending}
                        className="flex items-center gap-1.5 rounded-full border border-line px-4 py-2 text-sm text-ink transition hover:border-accent disabled:opacity-40"
                      >
                        <XCircle className="h-4 w-4" />
                        Batalkan
                      </button>
                    )}
                  </div>
                )}
              </div>
            );
          })}
          {transfersQuery.data.length === 0 && <p className="text-sm text-ink-muted">Belum ada transfer.</p>}
        </div>
      )}

      {showForm && (
        <CreateTransferModal
          branchId={branchId}
          branches={branchesQuery.data ?? []}
          onClose={() => setShowForm(false)}
          onCreated={() => {
            setShowForm(false);
            invalidate();
          }}
        />
      )}
    </div>
  );
}

function CreateTransferModal({
  branchId,
  branches,
  onClose,
  onCreated,
}: {
  branchId: string;
  branches: Branch[];
  onClose: () => void;
  onCreated: () => void;
}) {
  const [toBranchId, setToBranchId] = useState("");
  const [items, setItems] = useState<TransferItem[]>([]);
  const [search, setSearch] = useState("");
  const [searchSubmitted, setSearchSubmitted] = useState("");
  const [error, setError] = useState<string | null>(null);

  const searchQuery = useQuery({
    queryKey: ["inventory", "product-search", branchId, searchSubmitted],
    queryFn: () => apiClient.listProducts({ search: searchSubmitted, branch: branchId, limit: 8 }),
    enabled: searchSubmitted.length > 0,
  });

  const createMutation = useMutation({
    mutationFn: async () => apiClient.adminCreateTransfer(branchId, { to_branch_id: toBranchId, items }),
    onSuccess: onCreated,
    onError: (err) => setError(err instanceof ApiError ? err.message : "Gagal membuat transfer."),
  });

  async function addProduct(product: ProductListItem) {
    const detail = await apiClient.getProduct(product.slug, branchId);
    const variant = detail.variants[0];
    if (!variant) return;
    setItems((prev) => {
      const existing = prev.find((i) => i.product_variant_id === variant.id);
      if (existing) {
        return prev.map((i) => (i.product_variant_id === variant.id ? { ...i, quantity: i.quantity + 1 } : i));
      }
      return [...prev, { product_variant_id: variant.id, product_name: product.name, variant_name: variant.name, quantity: 1 }];
    });
  }

  function updateQuantity(variantId: string, quantity: number) {
    setItems((prev) => prev.map((i) => (i.product_variant_id === variantId ? { ...i, quantity } : i)));
  }

  function removeItem(variantId: string) {
    setItems((prev) => prev.filter((i) => i.product_variant_id !== variantId));
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <div className="max-h-[90vh] w-full max-w-lg overflow-y-auto animate-scale-in rounded-2xl border border-line bg-surface p-6 shadow-xl">
        <h2 className="font-display text-lg text-ink">Buat Transfer Stok</h2>

        <div className="mt-4 flex flex-col gap-3">
          <div className="flex flex-col gap-1.5">
            <label className="text-sm text-ink-muted">Kirim ke Cabang</label>
            <select value={toBranchId} onChange={(e) => setToBranchId(e.target.value)} className={inputClass}>
              <option value="">Pilih cabang tujuan...</option>
              {branches.filter((b) => b.id !== branchId).map((b) => (
                <option key={b.id} value={b.id}>
                  {b.name}
                </option>
              ))}
            </select>
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
              placeholder="Cari produk untuk ditransfer..."
              className={`${inputClass} flex-1`}
            />
            <button type="submit" className="rounded-full border border-line px-4 py-2 text-sm text-ink">
              Cari
            </button>
          </form>

          {searchQuery.data && (
            <div className="flex flex-col gap-2">
              {searchQuery.data.items.map((p) => (
                <button
                  key={p.id}
                  onClick={() => addProduct(p)}
                  className="flex items-center justify-between rounded-2xl border border-line p-3 text-left text-sm transition hover:border-accent"
                >
                  <span className="text-ink">{p.name}</span>
                  <span className="text-ink-muted">Stok: {p.branch_stock ?? "-"}</span>
                </button>
              ))}
            </div>
          )}

          {items.length > 0 && (
            <div className="flex flex-col divide-y divide-line rounded-2xl border border-line">
              {items.map((item) => (
                <div key={item.product_variant_id} className="flex items-center justify-between gap-2 p-3 text-sm">
                  <span className="text-ink">
                    {item.product_name} ({item.variant_name})
                  </span>
                  <div className="flex items-center gap-2">
                    <input
                      type="number"
                      min={1}
                      value={item.quantity}
                      onChange={(e) => updateQuantity(item.product_variant_id, Math.max(1, Number(e.target.value)))}
                      className="w-16 rounded-full border border-line bg-background px-2 py-1 text-center text-ink"
                    />
                    <button onClick={() => removeItem(item.product_variant_id)} className="text-red-400">
                      Hapus
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}

          {error && <p className="text-sm text-red-400">{error}</p>}

          <div className="mt-2 flex justify-end gap-3">
            <button type="button" onClick={onClose} className="rounded-full border border-line px-4 py-2 text-sm text-ink">
              Batal
            </button>
            <button
              disabled={!toBranchId || items.length === 0 || createMutation.isPending}
              onClick={() => createMutation.mutate()}
              className="rounded-full bg-accent px-4 py-2.5 text-sm font-medium text-background transition hover:opacity-90 disabled:opacity-40"
            >
              {createMutation.isPending ? "Membuat..." : "Buat Transfer"}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
