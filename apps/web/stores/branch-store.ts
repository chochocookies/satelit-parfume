"use client";

import { create } from "zustand";
import { persist } from "zustand/middleware";

export type SelectedBranch = {
  id: string;
  name: string;
  slug: string;
};

type BranchState = {
  selectedBranch: SelectedBranch | null;
  // localStorage only exists client-side, so the very first render (SSR
  // and the client's first paint, before hydration finishes) always sees
  // selectedBranch: null. Components that care about "is a branch
  // selected" should check hasHydrated first — see components/branch-selector.tsx
  // — otherwise a returning visitor briefly flashes "no branch" before
  // their saved one loads, or worse, mismatches what the server rendered.
  hasHydrated: boolean;
  setBranch: (branch: SelectedBranch) => void;
  clearBranch: () => void;
  setHasHydrated: (value: boolean) => void;
};

// Persisted to localStorage — section 10: "the selected branch must
// persist using: cookie / local storage / user preference when logged
// in." This is the local-storage half. Syncing it to a logged-in
// customer's saved preference is Phase 6+ work, once a customer-profile
// endpoint exists to store it against (section 38's "Preferred Store").
export const useBranchStore = create<BranchState>()(
  persist(
    (set) => ({
      selectedBranch: null,
      hasHydrated: false,
      setBranch: (branch) => set({ selectedBranch: branch }),
      clearBranch: () => set({ selectedBranch: null }),
      setHasHydrated: (value) => set({ hasHydrated: value }),
    }),
    {
      name: "satelit-parfume-branch",
      onRehydrateStorage: () => (state) => {
        state?.setHasHydrated(true);
      },
    },
  ),
);
