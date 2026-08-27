const STATUS_LABELS: Record<string, string> = {
  PENDING_PAYMENT: "Menunggu Bayar",
  PAID: "Dibayar",
  PROCESSING: "Diproses",
  PACKED: "Dikemas",
  READY_FOR_PICKUP: "Siap Diambil",
  SHIPPED: "Dikirim",
  DELIVERED: "Terkirim",
  COMPLETED: "Selesai",
  CANCELLED: "Dibatalkan",
  REFUNDED: "Dana Dikembalikan",
  EXPIRED: "Kedaluwarsa",
};

// Grouped by what the status means for the business, not by exact
// status name — every terminal "didn't work out" status (CANCELLED,
// REFUNDED, EXPIRED) reads the same visual weight, distinct from the
// in-progress amber/blue states and the successful green ones.
const STATUS_STYLES: Record<string, string> = {
  PENDING_PAYMENT: "bg-amber-400/10 text-amber-300 border-amber-400/30",
  PAID: "bg-sky-400/10 text-sky-300 border-sky-400/30",
  PROCESSING: "bg-sky-400/10 text-sky-300 border-sky-400/30",
  PACKED: "bg-indigo-400/10 text-indigo-300 border-indigo-400/30",
  READY_FOR_PICKUP: "bg-teal-400/10 text-teal-300 border-teal-400/30",
  SHIPPED: "bg-teal-400/10 text-teal-300 border-teal-400/30",
  DELIVERED: "bg-emerald-400/10 text-emerald-300 border-emerald-400/30",
  COMPLETED: "bg-emerald-400/10 text-emerald-300 border-emerald-400/30",
  CANCELLED: "bg-red-400/10 text-red-300 border-red-400/30",
  REFUNDED: "bg-red-400/10 text-red-300 border-red-400/30",
  EXPIRED: "bg-ink-muted/10 text-ink-muted border-line",
};

export function StatusBadge({ status }: { status: string }) {
  const style = STATUS_STYLES[status] ?? "bg-ink-muted/10 text-ink-muted border-line";
  const label = STATUS_LABELS[status] ?? status;

  return (
    <span className={`inline-flex items-center rounded-full border px-3 py-1 text-xs font-medium ${style}`}>
      {label}
    </span>
  );
}
