"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { AlertTriangle, Loader2, Package, Receipt, ShoppingBag, Store, Users } from "lucide-react";

import { apiClient } from "@/lib/api-client";
import { formatRupiah } from "@/lib/format";
import { StatCard } from "@/components/admin/stat-card";
import { StatusBadge } from "@/components/admin/status-badge";

export default function AdminDashboardPage() {
  const statsQuery = useQuery({
    queryKey: ["admin", "dashboard-stats"],
    queryFn: () => apiClient.adminDashboardStats(),
  });

  const recentOrdersQuery = useQuery({
    queryKey: ["admin", "orders", { limit: 5 }],
    queryFn: () => apiClient.adminListOrders({ limit: 5 }),
  });

  const branchesQuery = useQuery({
    queryKey: ["branches"],
    queryFn: () => apiClient.listBranches(),
  });

  const branchName = (branchId: string) => branchesQuery.data?.find((b) => b.id === branchId)?.name ?? branchId;

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h1 className="font-display text-2xl text-ink">Ringkasan</h1>
        <p className="mt-1 text-sm text-ink-muted">Semua angka di bawah dihitung langsung dari data saat ini.</p>
      </div>

      {statsQuery.isLoading && (
        <div className="flex items-center gap-2 text-sm text-ink-muted">
          <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
          Memuat statistik...
        </div>
      )}
      {statsQuery.isError && <p className="text-sm text-red-400">Gagal memuat statistik dashboard.</p>}

      {statsQuery.data && (
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-5">
          <StatCard
            label="Total Pendapatan"
            value={formatRupiah(statsQuery.data.revenue)}
            hint="Dari pesanan yang sudah dibayar"
            icon={Receipt}
          />
          <StatCard label="Total Pesanan" value={String(statsQuery.data.total_orders)} icon={ShoppingBag} />
          <StatCard label="Total Produk" value={String(statsQuery.data.total_products)} icon={Package} />
          <StatCard label="Total Cabang" value={String(statsQuery.data.total_branches)} icon={Store} />
          <StatCard label="Total Staf" value={String(statsQuery.data.total_staff)} icon={Users} />
        </div>
      )}

      {statsQuery.data && statsQuery.data.low_stock_count > 0 && (
        <div className="flex items-center gap-2 rounded-2xl border border-amber-400/30 bg-amber-400/10 p-4 text-sm text-amber-300">
          <AlertTriangle className="h-4 w-4 shrink-0" aria-hidden="true" />
          {statsQuery.data.low_stock_count} item stok cabang sudah mencapai atau di bawah batas minimum.
        </div>
      )}

      {statsQuery.data && (
        <div className="rounded-2xl border border-line bg-surface p-5">
          <h2 className="text-sm font-medium text-ink">Pesanan per Status</h2>
          <div className="mt-3 flex flex-wrap gap-2">
            {Object.entries(statsQuery.data.order_counts).map(([status, count]) => (
              <div key={status} className="flex items-center gap-2 rounded-full border border-line px-3 py-1.5">
                <StatusBadge status={status} />
                <span className="text-sm text-ink">{count}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      <div className="rounded-2xl border border-line bg-surface p-5">
        <div className="flex items-center justify-between">
          <h2 className="text-sm font-medium text-ink">Pesanan Terbaru</h2>
          <Link href="/admin/orders" className="text-sm text-accent hover:opacity-80">
            Lihat semua
          </Link>
        </div>

        {recentOrdersQuery.isLoading && <p className="mt-3 text-sm text-ink-muted">Memuat...</p>}
        {recentOrdersQuery.isError && <p className="mt-3 text-sm text-red-400">Gagal memuat pesanan terbaru.</p>}

        {recentOrdersQuery.data && recentOrdersQuery.data.items.length === 0 && (
          <p className="mt-3 text-sm text-ink-muted">Belum ada pesanan.</p>
        )}

        {recentOrdersQuery.data && recentOrdersQuery.data.items.length > 0 && (
          <div className="mt-3 flex flex-col divide-y divide-line">
            {recentOrdersQuery.data.items.map((order) => (
              <Link
                key={order.id}
                href={`/admin/orders/${order.id}`}
                className="flex items-center justify-between gap-3 py-3 text-sm transition hover:opacity-80"
              >
                <div>
                  <p className="text-ink">{order.order_number}</p>
                  <p className="text-xs text-ink-muted">
                    {branchName(order.branch_id)} · {order.guest_name ?? order.recipient_name ?? "-"}
                  </p>
                </div>
                <div className="flex items-center gap-3">
                  <span className="text-ink">{formatRupiah(order.total)}</span>
                  <StatusBadge status={order.status} />
                </div>
              </Link>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
