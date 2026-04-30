"use client";

import type { ReactNode } from "react";
import { SessionProvider } from "next-auth/react";

import { QueryProvider } from "@/components/providers/query-provider";
import { Toaster } from "@/components/ui/sonner";

export function AppProviders({ children }: { children: ReactNode }) {
  return (
    <SessionProvider>
      <QueryProvider>
        {children}
        <Toaster />
      </QueryProvider>
    </SessionProvider>
  );
}
