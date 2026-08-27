import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

// Standard shadcn/ui helper: merges conditional class names (clsx) and
// then resolves conflicting Tailwind utility classes (tailwind-merge).
// Kept here so `npx shadcn add <component>` works out of the box in
// later phases without needing this file re-created by hand.
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
