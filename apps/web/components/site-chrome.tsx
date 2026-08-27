"use client";

import { usePathname } from "next/navigation";

import { SiteHeader } from "@/components/site-header";
import { SiteFooter } from "@/components/site-footer";

// SiteChrome exists for one reason: app/layout.tsx is a Server Component
// (it needs to stay one, for metadata/font setup), so it can't call
// usePathname() itself to decide whether the current route wants the
// customer-facing header/footer. /admin/* and /pos/* build their own
// shells instead (app/admin/layout.tsx, app/pos/layout.tsx) — a staff
// dashboard or a cashier's till screen showing the shop's cart button
// and branch selector would just be confusing.
//
// For every other path this renders exactly what RootLayout used to
// render inline (SiteHeader, a flex-1 wrapper, SiteFooter) — behavior for
// every existing customer-facing page is unchanged.
export function SiteChrome({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const hasOwnShell = pathname?.startsWith("/admin") || pathname?.startsWith("/pos") || false;

  if (hasOwnShell) {
    return <>{children}</>;
  }

  return (
    <>
      <SiteHeader />
      <div className="flex-1">{children}</div>
      <SiteFooter />
    </>
  );
}
