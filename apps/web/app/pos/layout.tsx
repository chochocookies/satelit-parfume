"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { LayoutDashboard, Loader2, LogOut, Warehouse } from "lucide-react";

import { apiClient } from "@/lib/api-client";
import { useAuthStore } from "@/stores/auth-store";

// POS is deliberately its own guard, separate from /admin's: a CASHIER
// needs this screen every shift but has no business in the back-office
// dashboard, so the allowed-role set here is wider than
// app/admin/layout.tsx's ALLOWED_ROLES.
const ALLOWED_ROLES = ["SUPER_ADMIN", "ADMIN", "BRANCH_MANAGER", "CASHIER"];

export default function PosLayout({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const { hasHydrated, accessToken, subject, clear } = useAuthStore();
  const [checked, setChecked] = useState(false);
  const [allowed, setAllowed] = useState(false);

  useEffect(() => {
    if (!hasHydrated) {
      return;
    }
    if (!accessToken) {
      router.replace("/admin/login");
      return;
    }
    apiClient
      .me(accessToken)
      .then((me) => {
        if (!ALLOWED_ROLES.some((role) => me.roles.includes(role))) {
          clear();
          router.replace("/admin/login");
          return;
        }
        setAllowed(true);
      })
      .catch(() => {
        clear();
        router.replace("/admin/login");
      })
      .finally(() => setChecked(true));
  }, [hasHydrated, accessToken, router, clear]);

  if (!hasHydrated || !checked) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background">
        <Loader2 className="h-5 w-5 animate-spin text-ink-muted" aria-hidden="true" />
      </div>
    );
  }

  if (!allowed) {
    return null;
  }

  async function handleLogout() {
    const { refreshToken } = useAuthStore.getState();
    if (refreshToken) {
      try {
        await apiClient.logout(refreshToken);
      } catch {
        // best-effort — the local session clears either way below
      }
    }
    clear();
    router.replace("/admin/login");
  }

  return (
    <div className="flex min-h-screen flex-col bg-background">
      <header className="flex items-center justify-between border-b border-line px-4 py-3 sm:px-6">
        <Link href="/pos" className="font-display text-lg text-ink">
          Satelit Parfume <span className="text-ink-muted">Kasir</span>
        </Link>
        <div className="flex items-center gap-3">
          <span className="hidden text-sm text-ink-muted sm:inline">{subject?.name}</span>
          <Link href="/inventory" className="flex items-center gap-1.5 text-sm text-accent transition hover:opacity-80">
            <Warehouse className="h-4 w-4" />
            <span className="hidden sm:inline">Inventaris</span>
          </Link>
          {(subject?.roles.includes("SUPER_ADMIN") || subject?.roles.includes("ADMIN")) && (
            <Link href="/admin" className="flex items-center gap-1.5 text-sm text-accent transition hover:opacity-80">
              <LayoutDashboard className="h-4 w-4" />
              <span className="hidden sm:inline">Dashboard</span>
            </Link>
          )}
          <button
            onClick={handleLogout}
            className="flex items-center gap-1.5 rounded-full border border-line px-4 py-2 text-sm text-ink transition hover:border-accent"
          >
            <LogOut className="h-4 w-4" />
            Keluar
          </button>
        </div>
      </header>
      <main className="flex-1 animate-fade-in p-4 sm:p-6">{children}</main>
    </div>
  );
}
