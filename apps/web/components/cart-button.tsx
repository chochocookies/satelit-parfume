"use client";

import { useRef, useState } from "react";
import { ShoppingBag } from "lucide-react";

import { useCart } from "@/hooks/use-cart";
import { useClickOutside } from "@/hooks/use-click-outside";
import { CartPanel } from "@/components/cart-panel";

export function CartButton() {
  const [isOpen, setIsOpen] = useState(false);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  useClickOutside([triggerRef, panelRef], () => setIsOpen(false));

  const { cart } = useCart();
  const itemCount = cart?.item_count ?? 0;

  return (
    <div className="relative">
      <button
        ref={triggerRef}
        onClick={() => setIsOpen((v) => !v)}
        aria-label="Buka keranjang"
        aria-expanded={isOpen}
        className="relative flex h-10 w-10 items-center justify-center rounded-full border border-line bg-surface text-ink transition hover:border-accent"
      >
        <ShoppingBag className="h-4 w-4" />
        {itemCount > 0 && (
          <span className="absolute -right-1 -top-1 flex h-4 min-w-4 items-center justify-center rounded-full bg-accent px-1 text-[10px] font-medium text-background">
            {itemCount}
          </span>
        )}
      </button>

      {isOpen && (
        <div ref={panelRef} className="absolute right-0 z-30 mt-2 w-96">
          <CartPanel onClose={() => setIsOpen(false)} />
        </div>
      )}
    </div>
  );
}
