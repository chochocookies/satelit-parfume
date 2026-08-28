"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";

import { apiClient } from "@/lib/api-client";
import { useAuthStore } from "@/stores/auth-store";

// Its own guard, separate from /admin's and /pos's — same reasoning as
// Phase 10's /pos: BRANCH_MANAGER and INVENTORY_STAFF need this section
// but have no business in the SUPER_ADMIN/ADMIN-only back-office pages
// (products, staff, cross-branch orders), which this role set can't
// actually call successfully anyway.
const ALLOWED_ROLES = ["SUPER_ADMIN", "ADMIN", "BRANCH_MANAGER", "INVENTORY_STAFF"];

const NAV_ITEMS = [
  { href: "/inventory", label: "Stok" },
  { href: "/inventory/transfers", label: "Transfer" },
  { href: "/inventory/opname", label: "Stock Opname" },
];

export default function InventoryLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
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
        <p className="text-sm text-ink-muted">Memuat...</p>
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
      <header className="border-b border-line">
        <div className="flex items-center justify-between px-4 py-4 sm:px-6">
          <Link href="/inventory" className="font-display text-lg text-ink">
            Satelit Parfume <span className="text-ink-muted">Inventaris</span>
          </Link>
          <div className="flex items-center gap-3">
            <span className="hidden text-sm text-ink-muted sm:inline">{subject?.name}</span>
            {(subject?.roles.includes("SUPER_ADMIN") || subject?.roles.includes("ADMIN")) && (
              <Link href="/admin" className="text-sm text-accent hover:opacity-80">
                Dashboard
              </Link>
            )}
            <Link href="/pos" className="text-sm text-accent hover:opacity-80">
              Kasir
            </Link>
            <button
              onClick={handleLogout}
              className="rounded-full border border-line px-4 py-2 text-sm text-ink transition hover:border-accent"
            >
              Keluar
            </button>
          </div>
        </div>
        <nav className="flex gap-1 overflow-x-auto px-4 pb-3 sm:px-6">
          {NAV_ITEMS.map((item) => {
            const active = item.href === "/inventory" ? pathname === "/inventory" : (pathname?.startsWith(item.href) ?? false);
            return (
              <Link
                key={item.href}
                href={item.href}
                className={`shrink-0 rounded-full px-4 py-2 text-sm transition ${
                  active ? "bg-accent text-background" : "text-ink-muted hover:text-ink"
                }`}
              >
                {item.label}
              </Link>
            );
          })}
        </nav>
      </header>
      <main className="flex-1 p-4 sm:p-6">{children}</main>
    </div>
  );
}
