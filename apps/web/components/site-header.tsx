"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";
import { Heart, LogOut, Menu, Search, User, X } from "lucide-react";

import { BranchSelector } from "@/components/branch-selector";
import { CartButton } from "@/components/cart-button";
import { useCustomerAuthStore } from "@/stores/customer-auth-store";
import { apiClient } from "@/lib/api-client";

// Deliberately minimal nav: Logo, Shop, search, branch selector, account,
// cart. Section 14 lists a fuller set (Collections, Find Your Scent,
// Wishlist) but most of those still lead to pages that don't exist yet
// — a nav link to nowhere is worse than no link. Wishlist is the first
// exception (Phase 12): it's customer-only the same way the account
// greeting itself is, so it sits next to that instead of in the
// everyone-sees-it Shop row.
export function SiteHeader() {
  const router = useRouter();
  const [query, setQuery] = useState("");
  const [mobileOpen, setMobileOpen] = useState(false);
  const { hasHydrated, customer, refreshToken, clear } = useCustomerAuthStore();

  function handleSearch(event: FormEvent) {
    event.preventDefault();
    router.push(query ? `/shop?search=${encodeURIComponent(query)}` : "/shop");
    setMobileOpen(false);
  }

  async function handleLogout() {
    if (refreshToken) {
      try {
        await apiClient.customerLogout(refreshToken);
      } catch {
        // best-effort — the local session clears either way below
      }
    }
    clear();
    setMobileOpen(false);
    router.push("/");
  }

  return (
    <header className="sticky top-0 z-30 border-b border-line bg-background/90 backdrop-blur">
      <div className="mx-auto flex max-w-6xl items-center gap-4 px-6 py-4">
        <Link href="/" className="shrink-0 font-display text-lg tracking-wide text-ink">
          SATELIT PARFUME
        </Link>

        <nav className="hidden items-center gap-6 text-sm text-ink-muted sm:flex">
          <Link href="/shop" className="transition hover:text-ink">
            Belanja
          </Link>
        </nav>

        <form
          onSubmit={handleSearch}
          className="ml-auto hidden max-w-xs flex-1 items-center gap-2 rounded-full border border-line bg-surface px-4 py-2 transition focus-within:border-accent sm:flex"
        >
          <Search className="h-4 w-4 shrink-0 text-ink-muted" />
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Cari parfum…"
            className="w-full bg-transparent text-sm text-ink placeholder:text-ink-muted focus:outline-none"
          />
        </form>

        <div className="hidden sm:block">
          <BranchSelector />
        </div>

        {/* Account + cart stay visible at every width — small, fixed-size
            controls, not something that needs the mobile menu's extra
            room the way a full store list does. */}
        <div className="ml-auto flex items-center gap-3 sm:ml-0">
          {hasHydrated && (
            <div className="hidden items-center gap-3 sm:flex">
              {customer ? (
                <>
                  <Link href="/wishlist" aria-label="Wishlist" className="text-ink-muted transition hover:text-ink">
                    <Heart className="h-4 w-4" />
                  </Link>
                  <span className="text-sm text-ink-muted">Halo, {customer.name.split(" ")[0]}</span>
                  <button
                    onClick={handleLogout}
                    aria-label="Keluar"
                    className="text-ink-muted transition hover:text-ink"
                  >
                    <LogOut className="h-4 w-4" />
                  </button>
                </>
              ) : (
                <Link href="/login" className="flex items-center gap-1.5 text-sm text-ink-muted transition hover:text-ink">
                  <User className="h-4 w-4" />
                  Masuk
                </Link>
              )}
            </div>
          )}

          <CartButton />

          <button
            className="sm:hidden"
            onClick={() => setMobileOpen((v) => !v)}
            aria-label={mobileOpen ? "Tutup menu" : "Buka menu"}
            aria-expanded={mobileOpen}
          >
            {mobileOpen ? <X className="h-5 w-5 text-ink" /> : <Menu className="h-5 w-5 text-ink" />}
          </button>
        </div>
      </div>

      {mobileOpen && (
        <div className="animate-fade-in-up space-y-4 border-t border-line px-6 py-4 sm:hidden">
          <form
            onSubmit={handleSearch}
            className="flex items-center gap-2 rounded-full border border-line bg-surface px-4 py-2"
          >
            <Search className="h-4 w-4 shrink-0 text-ink-muted" />
            <input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Cari parfum…"
              className="w-full bg-transparent text-sm text-ink placeholder:text-ink-muted focus:outline-none"
            />
          </form>
          <Link href="/shop" className="block text-sm text-ink" onClick={() => setMobileOpen(false)}>
            Belanja
          </Link>
          <BranchSelector />
          {hasHydrated &&
            (customer ? (
              <div className="flex items-center justify-between">
                <span className="text-sm text-ink-muted">Halo, {customer.name.split(" ")[0]}</span>
                <div className="flex items-center gap-3">
                  <Link
                    href="/wishlist"
                    className="flex items-center gap-1.5 text-sm text-ink"
                    onClick={() => setMobileOpen(false)}
                  >
                    <Heart className="h-4 w-4" />
                    Wishlist
                  </Link>
                  <button onClick={handleLogout} className="flex items-center gap-1.5 text-sm text-ink">
                    <LogOut className="h-4 w-4" />
                    Keluar
                  </button>
                </div>
              </div>
            ) : (
              <Link
                href="/login"
                className="flex items-center gap-1.5 text-sm text-ink"
                onClick={() => setMobileOpen(false)}
              >
                <User className="h-4 w-4" />
                Masuk
              </Link>
            ))}
        </div>
      )}
    </header>
  );
}
