import type { ReactNode } from "react";

import { Logo } from "@/components/layout/logo";

/**
 * Focused auth layout: brand mark on top, full-bleed centered card,
 * no sidebar. Used for sign-in and sign-up.
 */
export default function AuthLayout({ children }: { children: ReactNode }) {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-background px-4 py-10">
      <div className="mb-8 flex flex-col items-center gap-2">
        <Logo size={40} />
        <p className="text-sm text-muted-foreground">
          API Gateway. Secret Guard. Built for Safety.
        </p>
      </div>
      <div className="w-full max-w-sm">{children}</div>
    </div>
  );
}
