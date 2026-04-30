"use client";

import Image from "next/image";
import { useState } from "react";

import { cn } from "@/lib/utils";

/**
 * Renders the brand logo with a graceful fallback when
 * `assets/logo.png` is missing from the public directory. The
 * fallback paints a small navy shield with a blue keyhole — close
 * enough to the brand mark described in `assets/README.md` to read
 * as LeakShield without shipping a placeholder image.
 */
export function Logo({
  size = 32,
  withWordmark = false,
  className,
}: {
  size?: number;
  withWordmark?: boolean;
  className?: string;
}) {
  const [errored, setErrored] = useState(false);

  return (
    <span className={cn("inline-flex items-center gap-2", className)}>
      {errored ? (
        <ShieldFallback size={size} />
      ) : (
        <Image
          src="/logo.png"
          alt="LeakShield"
          width={size}
          height={size}
          priority
          className="rounded-md"
          onError={() => setErrored(true)}
        />
      )}
      {withWordmark ? (
        <span className="text-base font-semibold tracking-tight text-foreground">
          LeakShield
        </span>
      ) : null}
    </span>
  );
}

function ShieldFallback({ size }: { size: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      role="img"
      aria-label="LeakShield"
    >
      <path
        d="M12 2 4 5v6c0 5 3.5 9 8 11 4.5-2 8-6 8-11V5l-8-3z"
        fill="var(--brand-navy)"
      />
      <circle cx="12" cy="11" r="2" fill="var(--brand-blue)" />
      <rect x="11" y="11" width="2" height="5" fill="var(--brand-blue)" />
    </svg>
  );
}
