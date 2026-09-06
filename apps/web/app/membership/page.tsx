"use client";

import Link from "next/link";
import { Award } from "lucide-react";
import { useQuery } from "@tanstack/react-query";

import { apiClient } from "@/lib/api-client";
import { useCustomerAuthStore } from "@/stores/customer-auth-store";
import { formatRupiah } from "@/lib/format";

const tierLabel: Record<string, string> = { BRONZE: "Bronze", SILVER: "Silver", GOLD: "Gold" };

export default function MembershipPage() {
  const customerAccessToken = useCustomerAuthStore((s) => s.accessToken);
  const customerHydrated = useCustomerAuthStore((s) => s.hasHydrated);
  const isCustomer = customerHydrated && !!customerAccessToken;

  const statusQuery = useQuery({
    queryKey: ["membership"],
    queryFn: apiClient.getMembershipStatus,
    enabled: isCustomer,
  });

  if (customerHydrated && !isCustomer) {
    return (
      <main className="mx-auto max-w-2xl px-6 py-12">
        <div className="mx-auto max-w-sm rounded-2xl border border-line bg-surface p-6 text-center">
          <Award className="mx-auto h-6 w-6 text-ink-muted" aria-hidden="true" />
          <p className="mt-3 text-sm text-ink-muted">
            <Link href="/login" className="text-accent hover:underline">
              Masuk
            </Link>{" "}
            untuk melihat status membership kamu.
          </p>
        </div>
      </main>
    );
  }

  const status = statusQuery.data;
  // Progress toward the next tier's own absolute threshold — simpler
  // and just as honest as showing progress within the current tier's
  // band would be, and doesn't need this page to duplicate the
  // threshold table that only Service.statusFor (backend) actually
  // owns: nextTierMin = total_spent + amount_to_next_tier, so
  // total_spent / nextTierMin is exactly "how far there".
  const nextTierMin = status ? status.total_spent + (status.amount_to_next_tier ?? 0) : 0;
  const progressPct = status?.next_tier && nextTierMin > 0 ? (status.total_spent / nextTierMin) * 100 : 100;

  return (
    <main className="mx-auto max-w-2xl px-6 py-12">
      <h1 className="font-display text-2xl text-ink">Membership</h1>

      {statusQuery.isLoading && <p className="mt-6 text-sm text-ink-muted">Memuat status membership…</p>}

      {status && (
        <div className="mt-6 rounded-2xl border border-line bg-surface p-6">
          <div className="flex items-center gap-3">
            <div className="flex h-12 w-12 items-center justify-center rounded-full bg-accent/10">
              <Award className="h-6 w-6 text-accent" aria-hidden="true" />
            </div>
            <div>
              <p className="text-xs text-ink-muted">Tier kamu</p>
              <p className="font-display text-xl text-ink">{tierLabel[status.tier] ?? status.tier}</p>
            </div>
          </div>

          <p className="mt-4 text-sm text-ink-muted">
            Total belanja (pesanan selesai): <span className="text-ink">{formatRupiah(status.total_spent)}</span>
          </p>

          {status.next_tier && status.amount_to_next_tier !== undefined && (
            <div className="mt-3">
              <div className="h-2 w-full overflow-hidden rounded-full bg-background">
                <div
                  className="h-full rounded-full bg-accent"
                  style={{ width: `${Math.min(100, Math.max(0, progressPct))}%` }}
                />
              </div>
              <p className="mt-2 text-xs text-ink-muted">
                Belanja {formatRupiah(status.amount_to_next_tier)} lagi untuk naik ke tier{" "}
                {tierLabel[status.next_tier] ?? status.next_tier}.
              </p>
            </div>
          )}

          <div className="mt-6 border-t border-line pt-4">
            <p className="text-xs text-ink-muted">Keuntungan tier kamu</p>
            <ul className="mt-2 space-y-1.5">
              {status.perks.map((perk) => (
                <li key={perk} className="text-sm text-ink">
                  • {perk}
                </li>
              ))}
            </ul>
          </div>
        </div>
      )}
    </main>
  );
}
