// Section 85: prices are always whole Rupiah (BIGINT, never floating
// point) — this only ever receives an integer, and formats it the same
// way everywhere a price is shown.
export function formatRupiah(amount: number): string {
  return new Intl.NumberFormat("id-ID", {
    style: "currency",
    currency: "IDR",
    maximumFractionDigits: 0,
  }).format(amount);
}
