"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { apiClient, ApiError, type OpnameItem, type StockOpname } from "@/lib/api-client";
import { useBranchStore } from "@/stores/branch-store";
import { BranchPicker } from "@/components/staff/branch-picker";

export default function OpnamePage() {
  const { selectedBranch, hasHydrated } = useBranchStore();

  if (!hasHydrated) {
    return <p className="text-sm text-ink-muted">Memuat...</p>;
  }
  if (!selectedBranch) {
    return <BranchPicker title="Pilih Cabang" subtitle="Pilih cabang untuk melakukan stock opname." />;
  }
  return <OpnameWorkspace branchId={selectedBranch.id} branchName={selectedBranch.name} />;
}

function OpnameWorkspace({ branchId, branchName }: { branchId: string; branchName: string }) {
  const queryClient = useQueryClient();
  const [completedSummary, setCompletedSummary] = useState<StockOpname | null>(null);

  const currentQuery = useQuery({
    queryKey: ["admin", "opname-current", branchId],
    queryFn: () => apiClient.adminCurrentOpname(branchId),
  });

  const historyQuery = useQuery({
    queryKey: ["admin", "opname-history", branchId],
    queryFn: () => apiClient.adminListOpnames(branchId),
    enabled: !currentQuery.data,
  });

  const startMutation = useMutation({
    mutationFn: () => apiClient.adminStartOpname(branchId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["admin", "opname-current", branchId] }),
  });

  if (completedSummary) {
    return <OpnameSummary opname={completedSummary} onDone={() => setCompletedSummary(null)} />;
  }

  if (currentQuery.isLoading) {
    return <p className="text-sm text-ink-muted">Memuat...</p>;
  }

  if (!currentQuery.data) {
    const completedHistory = (historyQuery.data ?? []).filter((o) => o.status === "completed");
    return (
      <div className="mx-auto flex max-w-sm flex-col gap-6 text-center">
        <div>
          <h1 className="font-display text-xl text-ink">Stock Opname — {branchName}</h1>
          <p className="mt-1 text-sm text-ink-muted">
            Tidak ada penghitungan stok yang sedang berjalan. Mulai satu untuk mencocokkan stok fisik dengan catatan sistem.
          </p>
          <button
            onClick={() => startMutation.mutate()}
            disabled={startMutation.isPending}
            className="mt-4 rounded-full bg-accent px-4 py-2.5 text-sm font-medium text-background transition hover:opacity-90 disabled:opacity-40"
          >
            {startMutation.isPending ? "Memulai..." : "Mulai Stock Opname"}
          </button>
        </div>

        {completedHistory.length > 0 && (
          <div className="text-left">
            <h2 className="text-sm font-medium text-ink-muted">Riwayat</h2>
            <div className="mt-2 flex flex-col divide-y divide-line rounded-2xl border border-line bg-surface">
              {completedHistory.map((o) => (
                <div key={o.id} className="flex items-center justify-between px-4 py-3 text-sm">
                  <span className="text-ink-muted">{o.completed_at ? new Date(o.completed_at).toLocaleDateString("id-ID") : "-"}</span>
                  <span className="text-ink">
                    {o.items.filter((i) => i.counted_quantity !== undefined && i.counted_quantity !== i.system_quantity).length} selisih
                  </span>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    );
  }

  return (
    <CountingScreen
      branchId={branchId}
      branchName={branchName}
      opname={currentQuery.data}
      onCompleted={(summary) => {
        setCompletedSummary(summary);
        queryClient.invalidateQueries({ queryKey: ["admin", "opname-current", branchId] });
      }}
    />
  );
}

function CountingScreen({
  branchId,
  branchName,
  opname,
  onCompleted,
}: {
  branchId: string;
  branchName: string;
  opname: StockOpname;
  onCompleted: (summary: StockOpname) => void;
}) {
  const queryClient = useQueryClient();
  const [notes, setNotes] = useState("");
  const [error, setError] = useState<string | null>(null);

  const countMutation = useMutation({
    mutationFn: ({ itemId, counted }: { itemId: string; counted: number }) =>
      apiClient.adminCountOpnameItem(branchId, opname.id, itemId, counted),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["admin", "opname-current", branchId] }),
  });

  const completeMutation = useMutation({
    mutationFn: () => apiClient.adminCompleteOpname(branchId, opname.id, notes),
    onSuccess: onCompleted,
    onError: (err) => setError(err instanceof ApiError ? err.message : "Gagal menyelesaikan stock opname."),
  });

  const countedCount = opname.items.filter((i) => i.counted_quantity !== undefined).length;

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h1 className="font-display text-2xl text-ink">Stock Opname — {branchName}</h1>
        <p className="mt-1 text-sm text-ink-muted">
          {countedCount} dari {opname.items.length} item sudah dihitung. Item yang belum dihitung tidak akan disesuaikan.
        </p>
      </div>

      <div className="overflow-x-auto rounded-2xl border border-line bg-surface">
        <table className="w-full text-left text-sm">
          <thead>
            <tr className="border-b border-line text-ink-muted">
              <th className="px-4 py-3 font-medium">Produk</th>
              <th className="px-4 py-3 font-medium">Sistem</th>
              <th className="px-4 py-3 font-medium">Hasil Hitung</th>
            </tr>
          </thead>
          <tbody>
            {opname.items.map((item: OpnameItem) => (
              <tr key={item.id} className="border-b border-line last:border-0">
                <td className="px-4 py-3 text-ink">
                  {item.product_name} <span className="text-ink-muted">· {item.variant_name}</span>
                </td>
                <td className="px-4 py-3 text-ink-muted">{item.system_quantity}</td>
                <td className="px-4 py-3">
                  <input
                    type="number"
                    min={0}
                    defaultValue={item.counted_quantity ?? ""}
                    onBlur={(e) => {
                      if (e.target.value === "") return;
                      countMutation.mutate({ itemId: item.id, counted: Number(e.target.value) });
                    }}
                    placeholder="-"
                    className="w-24 rounded-full border border-line bg-background px-3 py-1.5 text-sm text-ink focus:border-accent focus:outline-none"
                  />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="rounded-2xl border border-line bg-surface p-4">
        <label className="text-sm text-ink-muted">Catatan Penyelesaian (opsional)</label>
        <input
          value={notes}
          onChange={(e) => setNotes(e.target.value)}
          className="mt-1.5 w-full rounded-full border border-line bg-background px-4 py-2 text-sm text-ink placeholder:text-ink-muted focus:border-accent focus:outline-none"
        />
        {error && <p className="mt-2 text-sm text-red-400">{error}</p>}
        <button
          onClick={() => completeMutation.mutate()}
          disabled={completeMutation.isPending}
          className="mt-3 rounded-full bg-accent px-4 py-2.5 text-sm font-medium text-background transition hover:opacity-90 disabled:opacity-40"
        >
          {completeMutation.isPending ? "Menyelesaikan..." : "Selesaikan Stock Opname"}
        </button>
      </div>
    </div>
  );
}

function OpnameSummary({ opname, onDone }: { opname: StockOpname; onDone: () => void }) {
  const discrepancies = opname.items.filter(
    (i) => i.counted_quantity !== undefined && i.counted_quantity !== i.system_quantity,
  );

  return (
    <div className="mx-auto flex max-w-md flex-col gap-4">
      <div className="rounded-2xl border border-line bg-surface p-5">
        <h1 className="font-display text-lg text-ink">Ringkasan Stock Opname</h1>
        <p className="mt-1 text-sm text-ink-muted">{discrepancies.length} item memiliki selisih dari catatan sistem.</p>

        {discrepancies.length > 0 && (
          <div className="mt-4 flex flex-col divide-y divide-line text-sm">
            {discrepancies.map((item) => {
              const delta = (item.counted_quantity ?? 0) - item.system_quantity;
              return (
                <div key={item.id} className="flex items-center justify-between py-2">
                  <div>
                    <p className="text-ink">{item.product_name}</p>
                    <p className="text-xs text-ink-muted">
                      Sistem: {item.system_quantity} → Hitung: {item.counted_quantity}
                    </p>
                  </div>
                  <span className={delta > 0 ? "text-emerald-400" : "text-red-400"}>
                    {delta > 0 ? "+" : ""}
                    {delta}
                  </span>
                </div>
              );
            })}
          </div>
        )}
      </div>
      <button
        onClick={onDone}
        className="rounded-full bg-accent px-4 py-2.5 text-sm font-medium text-background transition hover:opacity-90"
      >
        Selesai
      </button>
    </div>
  );
}
