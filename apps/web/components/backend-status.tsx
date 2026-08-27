"use client";

import { useQuery } from "@tanstack/react-query";
import { apiClient } from "@/lib/api-client";

// Live proof that the frontend can actually reach the Go API, which can
// reach Postgres and Redis — the concrete thing Phase 1 needs to prove,
// rendered instead of just claimed.
export function BackendStatus() {
  const { data, isLoading, isError } = useQuery({
    queryKey: ["health"],
    queryFn: apiClient.getHealth,
  });

  const state = isLoading ? "loading" : isError ? "error" : "ok";

  const label =
    state === "loading"
      ? "Menghubungi backend…"
      : state === "error"
        ? "Backend belum bisa dihubungi — jalankan API & database (lihat README)"
        : `API ${data?.api} · Database ${data?.database} · Redis ${data?.redis}`;

  const dotClass =
    state === "ok" ? "bg-emerald-400" : state === "error" ? "bg-red-400" : "bg-ink-muted";

  return (
    <div className="flex items-center gap-3 rounded-full border border-line bg-surface px-4 py-2 text-xs text-ink-muted sm:text-sm">
      <span
        className={`inline-flex h-2 w-2 shrink-0 rounded-full ${dotClass} ${state === "loading" ? "animate-pulse" : ""}`}
      />
      <span>{label}</span>
    </div>
  );
}
