"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { apiClient, ApiError, type InventoryItem, type StockMovement } from "@/lib/api-client";
import { formatRupiah } from "@/lib/format";
import { useBranchStore } from "@/stores/branch-store";
import { BranchPicker } from "@/components/staff/branch-picker";

const inputClass =
  "rounded-full border border-line bg-background px-4 py-2 text-sm text-ink placeholder:text-ink-muted focus:border-accent focus:outline-none";

export default function InventoryPage() {
  const { selectedBranch, hasHydrated } = useBranchStore();

  if (!hasHydrated) {
    return <p className="text-sm text-ink-muted">Memuat...</p>;
  }
  if (!selectedBranch) {
    return <BranchPicker title="Pilih Cabang" subtitle="Pilih cabang untuk mengelola stok." />;
  }
  return <StockList branchId={selectedBranch.id} branchName={selectedBranch.name} />;
}

function StockList({ branchId, branchName }: { branchId: string; branchName: string }) {
  const [receiveItem, setReceiveItem] = useState<InventoryItem | null>(null);
  const [adjustItem, setAdjustItem] = useState<InventoryItem | null>(null);
  const [historyItem, setHistoryItem] = useState<InventoryItem | null>(null);

  const itemsQuery = useQuery({
    queryKey: ["admin", "inventory", branchId],
    queryFn: () => apiClient.adminListInventory(branchId),
  });

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h1 className="font-display text-2xl text-ink">Stok — {branchName}</h1>
        <p className="mt-1 text-sm text-ink-muted">Terima barang baru, sesuaikan jumlah, atau lihat riwayat pergerakan stok.</p>
      </div>

      {itemsQuery.isLoading && <p className="text-sm text-ink-muted">Memuat stok...</p>}
      {itemsQuery.isError && <p className="text-sm text-red-400">Gagal memuat stok.</p>}

      {itemsQuery.data && (
        <div className="overflow-x-auto rounded-2xl border border-line bg-surface">
          <table className="w-full text-left text-sm">
            <thead>
              <tr className="border-b border-line text-ink-muted">
                <th className="px-4 py-3 font-medium">Produk</th>
                <th className="px-4 py-3 font-medium">Stok</th>
                <th className="px-4 py-3 font-medium">Tersedia</th>
                <th className="px-4 py-3 font-medium">Min.</th>
                <th className="px-4 py-3 font-medium">Harga</th>
                <th className="px-4 py-3 font-medium"></th>
              </tr>
            </thead>
            <tbody>
              {itemsQuery.data.map((item) => (
                <tr
                  key={item.id}
                  className={`border-b border-line last:border-0 ${item.minimum_stock > 0 && item.available_stock <= item.minimum_stock ? "bg-amber-400/5" : ""}`}
                >
                  <td className="px-4 py-3 text-ink">
                    {item.product_name}
                    <span className="text-ink-muted"> · {item.variant_name}</span>
                  </td>
                  <td className="px-4 py-3 text-ink">{item.stock_quantity}</td>
                  <td className="px-4 py-3 text-ink">{item.available_stock}</td>
                  <td className="px-4 py-3 text-ink-muted">{item.minimum_stock}</td>
                  <td className="px-4 py-3 text-ink-muted">{item.price ? formatRupiah(item.price) : "-"}</td>
                  <td className="px-4 py-3">
                    <div className="flex justify-end gap-3">
                      <button onClick={() => setReceiveItem(item)} className="text-accent hover:opacity-80">
                        Terima
                      </button>
                      <button onClick={() => setAdjustItem(item)} className="text-accent hover:opacity-80">
                        Sesuaikan
                      </button>
                      <button onClick={() => setHistoryItem(item)} className="text-ink-muted hover:text-ink">
                        Riwayat
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
              {itemsQuery.data.length === 0 && (
                <tr>
                  <td colSpan={6} className="px-4 py-8 text-center text-ink-muted">
                    Belum ada stok tercatat di cabang ini.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      {receiveItem && <ReceiveModal branchId={branchId} item={receiveItem} onClose={() => setReceiveItem(null)} />}
      {adjustItem && <AdjustModal branchId={branchId} item={adjustItem} onClose={() => setAdjustItem(null)} />}
      {historyItem && <HistoryModal branchId={branchId} item={historyItem} onClose={() => setHistoryItem(null)} />}
    </div>
  );
}

function ReceiveModal({ branchId, item, onClose }: { branchId: string; item: InventoryItem; onClose: () => void }) {
  const queryClient = useQueryClient();
  const [quantity, setQuantity] = useState("");
  const [note, setNote] = useState("");
  const [error, setError] = useState<string | null>(null);

  const mutation = useMutation({
    mutationFn: () => apiClient.adminReceiveStock(branchId, item.product_variant_id, { quantity: Number(quantity), note }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["admin", "inventory", branchId] });
      onClose();
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Gagal mencatat penerimaan stok."),
  });

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <div className="w-full max-w-sm rounded-2xl border border-line bg-surface p-6 shadow-xl">
        <h2 className="font-display text-lg text-ink">Terima Stok</h2>
        <p className="mt-1 text-sm text-ink-muted">
          {item.product_name} · {item.variant_name} — saat ini {item.stock_quantity}
        </p>

        <form
          onSubmit={(e) => {
            e.preventDefault();
            setError(null);
            mutation.mutate();
          }}
          className="mt-4 flex flex-col gap-3"
        >
          <div className="flex flex-col gap-1.5">
            <label className="text-sm text-ink-muted">Jumlah Diterima</label>
            <input required type="number" min={1} value={quantity} onChange={(e) => setQuantity(e.target.value)} className={inputClass} />
          </div>
          <div className="flex flex-col gap-1.5">
            <label className="text-sm text-ink-muted">Catatan (opsional)</label>
            <input value={note} onChange={(e) => setNote(e.target.value)} className={inputClass} placeholder="mis. dari supplier X" />
          </div>
          {error && <p className="text-sm text-red-400">{error}</p>}
          <div className="mt-2 flex justify-end gap-3">
            <button type="button" onClick={onClose} className="rounded-full border border-line px-4 py-2 text-sm text-ink">
              Batal
            </button>
            <button
              type="submit"
              disabled={mutation.isPending}
              className="rounded-full bg-accent px-4 py-2.5 text-sm font-medium text-background transition hover:opacity-90 disabled:opacity-40"
            >
              {mutation.isPending ? "Menyimpan..." : "Simpan"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

function AdjustModal({ branchId, item, onClose }: { branchId: string; item: InventoryItem; onClose: () => void }) {
  const queryClient = useQueryClient();
  const [delta, setDelta] = useState("");
  const [note, setNote] = useState("");
  const [error, setError] = useState<string | null>(null);

  const mutation = useMutation({
    mutationFn: () => apiClient.adminAdjustStock(branchId, item.product_variant_id, { quantity_change: Number(delta), note }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["admin", "inventory", branchId] });
      onClose();
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Gagal menyesuaikan stok."),
  });

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <div className="w-full max-w-sm rounded-2xl border border-line bg-surface p-6 shadow-xl">
        <h2 className="font-display text-lg text-ink">Sesuaikan Stok</h2>
        <p className="mt-1 text-sm text-ink-muted">
          {item.product_name} · {item.variant_name} — saat ini {item.stock_quantity}
        </p>

        <form
          onSubmit={(e) => {
            e.preventDefault();
            setError(null);
            mutation.mutate();
          }}
          className="mt-4 flex flex-col gap-3"
        >
          <div className="flex flex-col gap-1.5">
            <label className="text-sm text-ink-muted">Perubahan (boleh negatif, mis. -3 untuk rusak/hilang)</label>
            <input required type="number" value={delta} onChange={(e) => setDelta(e.target.value)} className={inputClass} />
          </div>
          <div className="flex flex-col gap-1.5">
            <label className="text-sm text-ink-muted">Alasan</label>
            <input required value={note} onChange={(e) => setNote(e.target.value)} className={inputClass} placeholder="mis. rusak saat pengiriman" />
          </div>
          {error && <p className="text-sm text-red-400">{error}</p>}
          <div className="mt-2 flex justify-end gap-3">
            <button type="button" onClick={onClose} className="rounded-full border border-line px-4 py-2 text-sm text-ink">
              Batal
            </button>
            <button
              type="submit"
              disabled={mutation.isPending}
              className="rounded-full bg-accent px-4 py-2.5 text-sm font-medium text-background transition hover:opacity-90 disabled:opacity-40"
            >
              {mutation.isPending ? "Menyimpan..." : "Simpan"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

const REASON_LABELS: Record<string, string> = {
  sale: "Penjualan",
  receive: "Penerimaan",
  adjust: "Penyesuaian",
  transfer_in: "Transfer Masuk",
  transfer_out: "Transfer Keluar",
  transfer_cancelled: "Transfer Dibatalkan",
  opname: "Stock Opname",
};

function HistoryModal({ branchId, item, onClose }: { branchId: string; item: InventoryItem; onClose: () => void }) {
  const movementsQuery = useQuery({
    queryKey: ["admin", "stock-movements", branchId, item.product_variant_id],
    queryFn: () => apiClient.adminStockMovements(branchId, item.product_variant_id),
  });

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <div className="max-h-[80vh] w-full max-w-md overflow-y-auto rounded-2xl border border-line bg-surface p-6 shadow-xl">
        <h2 className="font-display text-lg text-ink">Riwayat — {item.product_name}</h2>
        <p className="text-sm text-ink-muted">{item.variant_name}</p>

        {movementsQuery.isLoading && <p className="mt-4 text-sm text-ink-muted">Memuat...</p>}

        <div className="mt-4 flex flex-col divide-y divide-line">
          {movementsQuery.data?.map((m: StockMovement) => (
            <div key={m.id} className="flex items-center justify-between gap-3 py-2 text-sm">
              <div>
                <p className="text-ink">{REASON_LABELS[m.reason] ?? m.reason}</p>
                <p className="text-xs text-ink-muted">
                  {new Date(m.created_at).toLocaleString("id-ID")}
                  {m.actor_name && ` · ${m.actor_name}`}
                </p>
                {m.note && <p className="text-xs text-ink-muted">{m.note}</p>}
              </div>
              <span className={m.quantity_change > 0 ? "text-emerald-400" : "text-red-400"}>
                {m.quantity_change > 0 ? "+" : ""}
                {m.quantity_change}
              </span>
            </div>
          ))}
          {movementsQuery.data?.length === 0 && <p className="py-3 text-sm text-ink-muted">Belum ada pergerakan stok.</p>}
        </div>

        <div className="mt-6 flex justify-end">
          <button onClick={onClose} className="rounded-full border border-line px-4 py-2 text-sm text-ink">
            Tutup
          </button>
        </div>
      </div>
    </div>
  );
}
