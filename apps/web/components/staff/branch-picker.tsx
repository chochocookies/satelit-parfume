"use client";

import { useQuery } from "@tanstack/react-query";

import { apiClient, type Branch } from "@/lib/api-client";
import { useBranchStore } from "@/stores/branch-store";

// Shared by Phase 11's inventory/transfers/opname pages — the same
// "which branch am I working at" prompt Phase 10's POS page has, reused
// here rather than duplicated a third time. POS's own inline version
// (app/pos/page.tsx) is left as-is: touching already-verified Phase 10
// code for a cosmetic dedupe isn't worth the risk.
export function BranchPicker({ title, subtitle }: { title: string; subtitle: string }) {
  const setBranch = useBranchStore((state) => state.setBranch);
  const branchesQuery = useQuery({ queryKey: ["branches"], queryFn: () => apiClient.listBranches() });

  return (
    <div className="mx-auto max-w-sm">
      <h1 className="font-display text-xl text-ink">{title}</h1>
      <p className="mt-1 text-sm text-ink-muted">{subtitle}</p>
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
