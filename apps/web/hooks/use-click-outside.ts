"use client";

import { useEffect, type RefObject } from "react";

// Calls onOutside when a pointer event lands outside every element in
// refs, or when Escape is pressed. Shared by any dropdown/panel that
// should close on outside interaction — branch selector today, sort/filter
// menus as they show up.
export function useClickOutside(refs: RefObject<HTMLElement | null>[], onOutside: () => void) {
  useEffect(() => {
    function handlePointer(event: PointerEvent) {
      const target = event.target as Node;
      const isInside = refs.some((ref) => ref.current?.contains(target));
      if (!isInside) onOutside();
    }

    function handleKey(event: KeyboardEvent) {
      if (event.key === "Escape") onOutside();
    }

    document.addEventListener("pointerdown", handlePointer);
    document.addEventListener("keydown", handleKey);
    return () => {
      document.removeEventListener("pointerdown", handlePointer);
      document.removeEventListener("keydown", handleKey);
    };
  }, [refs, onOutside]);
}
