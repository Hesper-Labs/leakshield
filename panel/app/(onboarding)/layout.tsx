import type { ReactNode } from "react";

import { Logo } from "@/components/layout/logo";

/**
 * Full-bleed focused-flow layout. No sidebar, no topbar — the wizard
 * is meant to feel like a brand-new install dialog.
 */
export default function OnboardingLayout({ children }: { children: ReactNode }) {
  return (
    <div className="flex min-h-screen flex-col bg-background">
      <header className="flex h-14 items-center border-b border-border px-6">
        <Logo size={28} withWordmark />
      </header>
      <main className="flex flex-1 items-start justify-center px-6 py-10">
        <div className="w-full max-w-3xl">{children}</div>
      </main>
    </div>
  );
}
