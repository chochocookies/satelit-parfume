"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { Boxes, LayoutDashboard, Loader2, LogOut, Package, ShoppingCart, Store, Users, Warehouse } from "lucide-react";

import { apiClient } from "@/lib/api-client";
import { useAuthStore } from "@/stores/auth-store";

const NAV_ITEMS = [
  { href: "/admin", label: "Ringkasan", icon: LayoutDashboard },
  { href: "/admin/products", label: "Produk", icon: Package },
  { href: "/admin/orders", label: "Pesanan", icon: ShoppingCart },
  { href: "/admin/branches", label: "Cabang", icon: Store },
  { href: "/admin/staff", label: "Staf", icon: Users },
];

// This is the SUPER_ADMIN/ADMIN dashboard the roadmap calls Phase 9.
// BRANCH_MANAGER/CASHIER/INVENTORY_STAFF already have their own
// narrower, branch-scoped endpoints from earlier phases (inventory,
// branch orders) — a dashboard variant for them is future work, not
// this phase (see the root README's "Deliberately not in Phase 9").
const ALLOWED_ROLES = ["SUPER_ADMIN", "ADMIN"];

export default function AdminLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const { hasHydrated, accessToken, subject, clear } = useAuthStore();
  const [checked, setChecked] = useState(false);
  const [allowed, setAllowed] = useState(false);

  const isLoginPage = pathname === "/admin/login";

  useEffect(() => {
    if (isLoginPage || !hasHydrated) {
      return;
    }

    if (!accessToken) {
      router.replace("/admin/login");
      return;
    }

    // Re-confirms against the live record rather than trusting whatever
    // role list localStorage still remembers — same reasoning as the
    // backend's own GetSubject ("re-fetches the current, live record
    // rather than trusting the access token's claims"). A staff account
    // suspended or demoted since the last login shouldn't keep dashboard
    // access just because its old access token hasn't expired yet.
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
  }, [isLoginPage, hasHydrated, accessToken, router, clear]);

  if (isLoginPage) {
    return <>{children}</>;
  }

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
      <header className="border-b border-line">
        <div className="flex items-center justify-between px-4 py-4 sm:px-6">
          <Link href="/admin" className="font-display text-lg text-ink">
            Satelit Parfume <span className="text-ink-muted">Admin</span>
          </Link>
          <div className="flex items-center gap-3">
            <span className="hidden text-sm text-ink-muted sm:inline">{subject?.name}</span>
            <Link href="/inventory" className="flex items-center gap-1.5 text-sm text-accent transition hover:opacity-80">
              <Warehouse className="h-4 w-4" />
              <span className="hidden sm:inline">Inventaris</span>
            </Link>
            <Link href="/pos" className="flex items-center gap-1.5 text-sm text-accent transition hover:opacity-80">
              <Boxes className="h-4 w-4" />
              <span className="hidden sm:inline">Kasir</span>
            </Link>
            <button
              onClick={handleLogout}
              className="flex items-center gap-1.5 rounded-full border border-line px-4 py-2 text-sm text-ink transition hover:border-accent"
            >
              <LogOut className="h-4 w-4" />
              Keluar
            </button>
          </div>
        </div>
        <nav className="flex gap-1 overflow-x-auto px-4 pb-3 sm:px-6">
          {NAV_ITEMS.map((item) => {
            const active = item.href === "/admin" ? pathname === "/admin" : (pathname?.startsWith(item.href) ?? false);
            const Icon = item.icon;
            return (
              <Link
                key={item.href}
                href={item.href}
                className={`flex shrink-0 items-center gap-1.5 rounded-full px-4 py-2 text-sm transition ${
                  active ? "bg-accent text-background" : "text-ink-muted hover:text-ink"
                }`}
              >
                <Icon className="h-4 w-4" />
                {item.label}
              </Link>
            );
          })}
        </nav>
      </header>
      <main className="flex-1 animate-fade-in p-4 sm:p-6">{children}</main>
    </div>
  );
}
