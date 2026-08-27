"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";
import { Menu, Search, X } from "lucide-react";

import { BranchSelector } from "@/components/branch-selector";
import { CartButton } from "@/components/cart-button";

// Deliberately minimal nav: Logo, Shop, search, branch selector, cart.
// Section 14 lists a fuller set (Collections, Find Your Scent, Wishlist,
// Account) but those lead to pages/features that don't exist yet
// (wishlist is Phase 12, account needs a customer-auth UI that hasn't
// been built) — a nav link to nowhere is worse than no link.
export function SiteHeader() {
  const router = useRouter();
  const [query, setQuery] = useState("");
  const [mobileOpen, setMobileOpen] = useState(false);

  function handleSearch(event: FormEvent) {
    event.preventDefault();
    router.push(query ? `/shop?search=${encodeURIComponent(query)}` : "/shop");
    setMobileOpen(false);
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
          className="ml-auto hidden max-w-xs flex-1 items-center gap-2 rounded-full border border-line bg-surface px-4 py-2 sm:flex"
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

        {/* Unlike the branch selector, the cart stays visible at every
            width — it's a single icon+badge, not something that needs
            the mobile menu's extra room the way a full store list does. */}
        <div className="ml-auto flex items-center gap-3 sm:ml-0">
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
        <div className="space-y-4 border-t border-line px-6 py-4 sm:hidden">
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
        </div>
      )}
    </header>
  );
}
