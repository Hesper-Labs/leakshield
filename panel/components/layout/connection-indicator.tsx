"use client";

import { useTranslations } from "next-intl";

import { cn } from "@/lib/utils";

export type ConnectionState =
  | "connecting"
  | "connected"
  | "paused"
  | "reconnecting"
  | "error";

const STATE_COLORS: Record<ConnectionState, string> = {
  connecting: "text-muted-foreground",
  connected: "text-success",
  paused: "text-warning",
  reconnecting: "text-warning",
  error: "text-destructive",
};

export function ConnectionIndicator({
  state,
  className,
  hideLabel = false,
}: {
  state: ConnectionState;
  className?: string;
  hideLabel?: boolean;
}) {
  const t = useTranslations("logs");
  const labels: Record<ConnectionState, string> = {
    connecting: t("reconnecting"),
    connected: t("live"),
    paused: t("paused"),
    reconnecting: t("reconnecting"),
    error: "Disconnected",
  };

  return (
    <div className={cn("flex items-center gap-2 text-sm font-medium", className)}>
      <span className="relative inline-flex h-2.5 w-2.5">
        <span
          className={cn(
            "absolute inset-0 inline-flex rounded-full",
            STATE_COLORS[state],
            state === "connected" && "pulse-dot",
          )}
        />
        <span
          className={cn(
            "relative inline-flex h-2.5 w-2.5 rounded-full bg-current",
            STATE_COLORS[state],
          )}
        />
      </span>
      {hideLabel ? null : (
        <span className={cn(STATE_COLORS[state])}>{labels[state]}</span>
      )}
    </div>
  );
}
