"use client";

import { create } from "zustand";
import { persist } from "zustand/middleware";

type CartTokenState = {
  cartToken: string | null;
  hasHydrated: boolean;
  setCartToken: (token: string) => void;
  setHasHydrated: (value: boolean) => void;
};

// Holds only the guest cart's session token — actual cart contents are
// server state, fetched fresh via TanStack Query (see hooks/use-cart.ts),
// never duplicated here. A logged-in customer wouldn't need this at all
// (their cart is found via their access token instead) — but there's no
// login UI built yet for a customer to actually be logged in through
// (see the root README's Phase 6 notes), so in practice every cart today
// goes through this guest path.
export const useCartStore = create<CartTokenState>()(
  persist(
    (set) => ({
      cartToken: null,
      hasHydrated: false,
      setCartToken: (token) => set({ cartToken: token }),
      setHasHydrated: (value) => set({ hasHydrated: value }),
    }),
    {
      name: "satelit-parfume-cart-token",
      onRehydrateStorage: () => (state) => {
        state?.setHasHydrated(true);
      },
    },
  ),
);
