"use client";

import { useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Check, ChevronDown, LocateFixed, MapPin } from "lucide-react";

import { apiClient } from "@/lib/api-client";
import { useBranchStore } from "@/stores/branch-store";
import { useClickOutside } from "@/hooks/use-click-outside";
import { cn } from "@/lib/utils";

// Section 10's branch selector + section 11's "use my location" nearest-
// store suggestion. Geolocation is opt-in (a button, not requested on
// page load) and only re-sorts the list — picking a branch is always a
// deliberate click, never automatic, per section 11: "do not automatically
// change the selected branch without user confirmation."
export function BranchSelector() {
  const [isOpen, setIsOpen] = useState(false);
  const [coords, setCoords] = useState<{ lat: number; lng: number } | null>(null);
  const [locating, setLocating] = useState(false);
  const [locationError, setLocationError] = useState<string | null>(null);

  const triggerRef = useRef<HTMLButtonElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  useClickOutside([triggerRef, panelRef], () => setIsOpen(false));

  const selectedBranch = useBranchStore((s) => s.selectedBranch);
  const hasHydrated = useBranchStore((s) => s.hasHydrated);
  const setBranch = useBranchStore((s) => s.setBranch);

  const { data: branches, isLoading } = useQuery({
    queryKey: ["branches", coords],
    queryFn: () => apiClient.listBranches(coords ?? undefined),
    enabled: isOpen,
  });

  function useMyLocation() {
    if (!("geolocation" in navigator)) {
      setLocationError("Perangkat ini tidak mendukung deteksi lokasi.");
      return;
    }
    setLocating(true);
    setLocationError(null);
    navigator.geolocation.getCurrentPosition(
      (position) => {
        setCoords({ lat: position.coords.latitude, lng: position.coords.longitude });
        setLocating(false);
      },
      () => {
        setLocationError("Tidak bisa mengakses lokasi. Pilih toko manual di bawah.");
        setLocating(false);
      },
      { timeout: 8000 },
    );
  }

  const label = !hasHydrated || !selectedBranch ? "Pilih Toko" : selectedBranch.name;

  return (
    <div className="relative">
      <button
        ref={triggerRef}
        onClick={() => setIsOpen((v) => !v)}
        aria-expanded={isOpen}
        aria-haspopup="listbox"
        className="flex items-center gap-2 rounded-full border border-line bg-surface px-4 py-2 text-sm text-ink transition hover:border-accent"
      >
        <MapPin className="h-4 w-4 shrink-0 text-accent" />
        <span className="max-w-[9rem] truncate sm:max-w-[12rem]">{label}</span>
        <ChevronDown className="h-4 w-4 shrink-0 text-ink-muted" />
      </button>

      {isOpen && (
        <div
          ref={panelRef}
          role="listbox"
          className="absolute right-0 z-30 mt-2 w-80 rounded-2xl border border-line bg-surface p-3 shadow-xl"
        >
          <button
            onClick={useMyLocation}
            disabled={locating}
            className="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left text-sm text-accent transition hover:bg-background disabled:opacity-50"
          >
            <LocateFixed className="h-4 w-4" />
            {locating ? "Mencari lokasimu…" : "Gunakan lokasiku"}
          </button>
          {locationError && <p className="px-3 pb-1 text-xs text-red-400">{locationError}</p>}

          <div className="mt-1 max-h-72 space-y-1 overflow-y-auto">
            {isLoading && <p className="px-3 py-4 text-sm text-ink-muted">Memuat daftar toko…</p>}

            {!isLoading && branches?.length === 0 && (
              <p className="px-3 py-4 text-sm text-ink-muted">
                Belum ada cabang terdaftar. Tambahkan dulu lewat endpoint admin — lihat README.
              </p>
            )}

            {branches?.map((branch) => {
              const isSelected = selectedBranch?.id === branch.id;
              return (
                <button
                  key={branch.id}
                  onClick={() => {
                    setBranch({ id: branch.id, name: branch.name, slug: branch.slug });
                    setIsOpen(false);
                  }}
                  className={cn(
                    "flex w-full items-center justify-between gap-2 rounded-lg px-3 py-2 text-left text-sm transition hover:bg-background",
                    isSelected ? "text-accent" : "text-ink",
                  )}
                >
                  <span>
                    <span className="block">{branch.name}</span>
                    {(branch.city || branch.distance_km != null) && (
                      <span className="block text-xs text-ink-muted">
                        {branch.city}
                        {branch.distance_km != null && ` · ${branch.distance_km.toFixed(1)} km`}
                      </span>
                    )}
                  </span>
                  {isSelected && <Check className="h-4 w-4 shrink-0" />}
                </button>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}
