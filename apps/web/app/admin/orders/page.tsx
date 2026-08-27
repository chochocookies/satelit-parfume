"use client";

import { useState } from "react";
import Link from "next/link";
import { useQuery } from "@tanstack/react-query";

import { apiClient } from "@/lib/api-client";
import { formatRupiah } from "@/lib/format";
import { StatusBadge } from "@/components/admin/status-badge";

const STATUS_OPTIONS = [
  "PENDING_PAYMENT",
  "PAID",
  "PROCESSING",
  "PACKED",
  "READY_FOR_PICKUP",
  "SHIPPED",
  "DELIVERED",
  "COMPLETED",
  "CANCELLED",
  "REFUNDED",
  "EXPIRED",
];

export default function AdminOrdersPage() {
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const [branchId, setBranchId] = useState("");
  const [page, setPage] = useState(1);

  const branchesQuery = useQuery({
    queryKey: ["branches"],
    queryFn: () => apiClient.listBranches(),
  });

  const ordersQuery = useQuery({
    queryKey: ["admin", "orders", { search, status, branchId, page }],
    queryFn: () =>
      apiClient.adminListOrders({
        search: search || undefined,
        status: status || undefined,
        branch_id: branchId || undefined,
        page,
        limit: 20,
      }),
  });

  const branchName = (id: string) => branchesQuery.data?.find((b) => b.id === id)?.name ?? id;

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h1 className="font-display text-2xl text-ink">Pesanan</h1>
        <p className="mt-1 text-sm text-ink-muted">Semua pesanan dari semua cabang.</p>
      </div>

      <div className="flex flex-wrap gap-3">
        <input
          value={search}
          onChange={(e) => {
            setSearch(e.target.value);
            setPage(1);
          }}
          placeholder="Cari no. pesanan, nama, telepon..."
          className="min-w-[220px] flex-1 rounded-full border border-line bg-background px-4 py-2 text-sm text-ink placeholder:text-ink-muted focus:border-accent focus:outline-none"
        />
        <select
          value={status}
          onChange={(e) => {
            setStatus(e.target.value);
            setPage(1);
          }}
          className="rounded-full border border-line bg-background px-4 py-2 text-sm text-ink focus:border-accent focus:outline-none"
        >
          <option value="">Semua Status</option>
          {STATUS_OPTIONS.map((s) => (
            <option key={s} value={s}>
              {s}
            </option>
          ))}
        </select>
        <select
          value={branchId}
          onChange={(e) => {
            setBranchId(e.target.value);
            setPage(1);
          }}
          className="rounded-full border border-line bg-background px-4 py-2 text-sm text-ink focus:border-accent focus:outline-none"
        >
          <option value="">Semua Cabang</option>
          {branchesQuery.data?.map((b) => (
            <option key={b.id} value={b.id}>
              {b.name}
            </option>
          ))}
        </select>
      </div>

      {ordersQuery.isLoading && <p className="text-sm text-ink-muted">Memuat pesanan...</p>}
      {ordersQuery.isError && <p className="text-sm text-red-400">Gagal memuat pesanan.</p>}

      {ordersQuery.data && (
        <div className="overflow-x-auto rounded-2xl border border-line bg-surface">
          <table className="w-full text-left text-sm">
            <thead>
              <tr className="border-b border-line text-ink-muted">
                <th className="px-4 py-3 font-medium">No. Pesanan</th>
                <th className="px-4 py-3 font-medium">Cabang</th>
                <th className="px-4 py-3 font-medium">Pelanggan</th>
                <th className="px-4 py-3 font-medium">Total</th>
                <th className="px-4 py-3 font-medium">Status</th>
              </tr>
            </thead>
            <tbody>
              {ordersQuery.data.items.map((order) => (
                <tr key={order.id} className="border-b border-line last:border-0">
                  <td className="px-4 py-3">
                    <Link href={`/admin/orders/${order.id}`} className="text-accent hover:opacity-80">
                      {order.order_number}
                    </Link>
                  </td>
                  <td className="px-4 py-3 text-ink-muted">{branchName(order.branch_id)}</td>
                  <td className="px-4 py-3 text-ink-muted">{order.guest_name ?? order.recipient_name ?? "-"}</td>
                  <td className="px-4 py-3 text-ink">{formatRupiah(order.total)}</td>
                  <td className="px-4 py-3">
                    <StatusBadge status={order.status} />
                  </td>
                </tr>
              ))}
              {ordersQuery.data.items.length === 0 && (
                <tr>
                  <td colSpan={5} className="px-4 py-8 text-center text-ink-muted">
                    Tidak ada pesanan yang cocok.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      {ordersQuery.data && ordersQuery.data.total_pages > 1 && (
        <div className="flex items-center justify-center gap-4 text-sm">
          <button
            disabled={page <= 1}
            onClick={() => setPage((p) => p - 1)}
            className="rounded-full border border-line px-4 py-2 text-ink disabled:opacity-40"
          >
            Sebelumnya
          </button>
          <span className="text-ink-muted">
            Halaman {ordersQuery.data.page} dari {ordersQuery.data.total_pages}
          </span>
          <button
            disabled={page >= ordersQuery.data.total_pages}
            onClick={() => setPage((p) => p + 1)}
            className="rounded-full border border-line px-4 py-2 text-ink disabled:opacity-40"
          >
            Berikutnya
          </button>
        </div>
      )}
    </div>
  );
}
