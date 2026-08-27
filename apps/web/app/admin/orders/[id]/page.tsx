"use client";

import { useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { apiClient, ApiError } from "@/lib/api-client";
import { formatRupiah } from "@/lib/format";
import { StatusBadge } from "@/components/admin/status-badge";

// Mirrors internal/orders/transitions.go's status graph — kept here so
// the dropdown only ever offers a move the backend will actually accept.
// The backend still validates independently on submit (Service.UpdateStatus);
// this is purely so staff don't see options that would just bounce back
// as an error.
const NEXT_STATUSES: Record<string, string[]> = {
  PENDING_PAYMENT: ["PAID", "CANCELLED", "EXPIRED"],
  PAID: ["PROCESSING", "REFUNDED"],
  PROCESSING: ["PACKED", "REFUNDED"],
  PACKED: ["READY_FOR_PICKUP", "SHIPPED", "REFUNDED"],
  READY_FOR_PICKUP: ["COMPLETED", "REFUNDED"],
  SHIPPED: ["DELIVERED", "REFUNDED"],
  DELIVERED: ["COMPLETED", "REFUNDED"],
  COMPLETED: [],
  CANCELLED: [],
  REFUNDED: [],
  EXPIRED: [],
};

export default function AdminOrderDetailPage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const queryClient = useQueryClient();
  const [nextStatus, setNextStatus] = useState("");
  const [note, setNote] = useState("");
  const [error, setError] = useState<string | null>(null);

  const orderQuery = useQuery({
    queryKey: ["admin", "order", params.id],
    queryFn: () => apiClient.adminGetOrder(params.id),
  });

  const branchesQuery = useQuery({
    queryKey: ["branches"],
    queryFn: () => apiClient.listBranches(),
  });

  const updateStatus = useMutation({
    mutationFn: () => apiClient.adminUpdateOrderStatus(params.id, nextStatus, note || undefined),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["admin", "order", params.id] });
      queryClient.invalidateQueries({ queryKey: ["admin", "orders"] });
      setNextStatus("");
      setNote("");
      setError(null);
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Gagal memperbarui status."),
  });

  if (orderQuery.isLoading) {
    return <p className="text-sm text-ink-muted">Memuat pesanan...</p>;
  }
  if (orderQuery.isError || !orderQuery.data) {
    return <p className="text-sm text-red-400">Pesanan tidak ditemukan.</p>;
  }

  const order = orderQuery.data;
  const branch = branchesQuery.data?.find((b) => b.id === order.branch_id);
  const availableNext = NEXT_STATUSES[order.status] ?? [];

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <div>
          <button onClick={() => router.push("/admin/orders")} className="text-sm text-ink-muted hover:text-ink">
            ← Kembali ke Pesanan
          </button>
          <h1 className="mt-1 font-display text-2xl text-ink">{order.order_number}</h1>
        </div>
        <StatusBadge status={order.status} />
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <div className="rounded-2xl border border-line bg-surface p-5">
          <h2 className="text-sm font-medium text-ink">Info Pesanan</h2>
          <dl className="mt-3 flex flex-col gap-2 text-sm">
            <Row label="Cabang" value={branch?.name ?? order.branch_id} />
            <Row label="Tipe" value={order.order_type === "pickup" ? "Ambil di Toko" : "Diantar"} />
            <Row label="Nama" value={order.guest_name ?? order.recipient_name ?? "-"} />
            <Row label="Telepon" value={order.guest_phone ?? order.recipient_phone ?? "-"} />
            {order.address_line && <Row label="Alamat" value={`${order.address_line}${order.city ? `, ${order.city}` : ""}`} />}
            {order.notes && <Row label="Catatan" value={order.notes} />}
            <Row label="Dibuat" value={new Date(order.created_at).toLocaleString("id-ID")} />
          </dl>
        </div>

        <div className="rounded-2xl border border-line bg-surface p-5">
          <h2 className="text-sm font-medium text-ink">Item</h2>
          <div className="mt-3 flex flex-col divide-y divide-line text-sm">
            {order.items.map((item) => (
              <div key={item.id} className="flex items-center justify-between py-2">
                <div>
                  <p className="text-ink">{item.product_name}</p>
                  <p className="text-xs text-ink-muted">
                    {item.variant_name} × {item.quantity}
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
        </div>
      </div>

      <div className="rounded-2xl border border-line bg-surface p-5">
        <h2 className="text-sm font-medium text-ink">Riwayat Status</h2>
        <div className="mt-3 flex flex-col gap-2 text-sm">
          {order.status_history.map((event, i) => (
            <div key={i} className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <StatusBadge status={event.status} />
                {event.note && <span className="text-ink-muted">{event.note}</span>}
              </div>
              <span className="text-xs text-ink-muted">{new Date(event.created_at).toLocaleString("id-ID")}</span>
            </div>
          ))}
        </div>
      </div>

      {availableNext.length > 0 && (
        <div className="rounded-2xl border border-line bg-surface p-5">
          <h2 className="text-sm font-medium text-ink">Perbarui Status</h2>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              if (nextStatus) updateStatus.mutate();
            }}
            className="mt-3 flex flex-wrap items-end gap-3"
          >
            <div className="flex flex-col gap-1.5">
              <label className="text-sm text-ink-muted">Status Baru</label>
              <select
                required
                value={nextStatus}
                onChange={(e) => setNextStatus(e.target.value)}
                className="rounded-full border border-line bg-background px-4 py-2 text-sm text-ink focus:border-accent focus:outline-none"
              >
                <option value="">Pilih status...</option>
                {availableNext.map((s) => (
                  <option key={s} value={s}>
                    {s}
                  </option>
                ))}
              </select>
            </div>
            <div className="flex flex-1 flex-col gap-1.5">
              <label className="text-sm text-ink-muted">Catatan (opsional)</label>
              <input
                value={note}
                onChange={(e) => setNote(e.target.value)}
                className="w-full rounded-full border border-line bg-background px-4 py-2 text-sm text-ink placeholder:text-ink-muted focus:border-accent focus:outline-none"
              />
            </div>
            <button
              type="submit"
              disabled={!nextStatus || updateStatus.isPending}
              className="rounded-full bg-accent px-4 py-2.5 text-sm font-medium text-background transition hover:opacity-90 disabled:opacity-40"
            >
              {updateStatus.isPending ? "Menyimpan..." : "Perbarui"}
            </button>
          </form>
          {error && <p className="mt-2 text-sm text-red-400">{error}</p>}
        </div>
      )}
    </div>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between gap-4">
      <dt className="text-ink-muted">{label}</dt>
      <dd className="text-right text-ink">{value}</dd>
    </div>
  );
}
