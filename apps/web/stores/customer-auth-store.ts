"use client";

import { create } from "zustand";
import { persist } from "zustand/middleware";

import type { AdminSubject } from "@/lib/api-client";

// Deliberately its own store, not a reuse of stores/auth-store.ts (staff
// sessions) — customers and staff are two completely separate account
// systems on the backend (internal/customers vs internal/users, per
// internal/auth's own package doc comment), so keeping their tokens in
// separate stores means a customer's session can never accidentally be
// sent to a staff-only endpoint, or vice versa, just because both
// happened to share one object.
type CustomerAuthState = {
  accessToken: string | null;
  refreshToken: string | null;
  customer: AdminSubject | null;
  hasHydrated: boolean;
  setSession: (accessToken: string, refreshToken: string, customer: AdminSubject) => void;
  clear: () => void;
  setHasHydrated: (value: boolean) => void;
};

export const useCustomerAuthStore = create<CustomerAuthState>()(
  persist(
    (set) => ({
      accessToken: null,
      refreshToken: null,
      customer: null,
      hasHydrated: false,
      setSession: (accessToken, refreshToken, customer) => set({ accessToken, refreshToken, customer }),
      clear: () => set({ accessToken: null, refreshToken: null, customer: null }),
      setHasHydrated: (value) => set({ hasHydrated: value }),
    }),
    {
      name: "satelit-parfume-customer-auth",
      onRehydrateStorage: () => (state) => {
        state?.setHasHydrated(true);
      },
    },
  ),
);
