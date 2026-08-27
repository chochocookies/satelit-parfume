"use client";

import { create } from "zustand";
import { persist } from "zustand/middleware";

import type { AdminSubject } from "@/lib/api-client";

type AuthState = {
  accessToken: string | null;
  refreshToken: string | null;
  subject: AdminSubject | null;
  // Same hydration-flash concern as stores/branch-store.ts: localStorage
  // only exists client-side, so the very first render always sees
  // subject: null even for a returning, already-logged-in staff member.
  // app/admin/layout.tsx waits for hasHydrated before deciding whether
  // to redirect to /admin/login, so a real session doesn't get bounced
  // by its own loading state.
  hasHydrated: boolean;
  setSession: (accessToken: string, refreshToken: string, subject: AdminSubject) => void;
  clear: () => void;
  setHasHydrated: (value: boolean) => void;
};

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      accessToken: null,
      refreshToken: null,
      subject: null,
      hasHydrated: false,
      setSession: (accessToken, refreshToken, subject) => set({ accessToken, refreshToken, subject }),
      clear: () => set({ accessToken: null, refreshToken: null, subject: null }),
      setHasHydrated: (value) => set({ hasHydrated: value }),
    }),
    {
      name: "satelit-parfume-admin-auth",
      onRehydrateStorage: () => (state) => {
        state?.setHasHydrated(true);
      },
    },
  ),
);
