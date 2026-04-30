"use client";

import { useEffect, useRef, useState } from "react";

import type { ConnectionState } from "@/components/layout/connection-indicator";

export interface AuditLogEntry {
  id: string;
  timestamp: string;
  user: string;
  provider: string;
  model: string;
  verdict: "ALLOW" | "MASK" | "BLOCK" | "ESCALATE";
  latencyMs?: number;
  category?: string;
  // The full row may contain richer fields once the gateway settles
  // its OpenAPI shape; we keep a typed subset here and let the rest
  // flow through `extra`.
  extra?: Record<string, unknown>;
}

interface UseLogStreamOptions {
  /** When true the hook stops appending new entries (still reading the
   *  socket so the buffer doesn't back-pressure). Set to true when the
   *  user scrolls away from the top of the live table. */
  paused?: boolean;
  /** Cap on how many entries to retain in memory. */
  bufferSize?: number;
  /** Optional query string forwarded to /api/stream/logs. */
  filter?: string;
}

/**
 * Subscribes to the panel's SSE proxy at `/api/stream/logs`, which in
 * turn proxies the gateway's `/admin/v1/stream/logs` feed with the
 * caller's bearer token attached server-side. Entries are appended in
 * arrival order; pause and buffer-cap are handled here so call sites
 * just consume the resulting array.
 */
export function useLogStream({
  paused = false,
  bufferSize = 500,
  filter,
}: UseLogStreamOptions = {}) {
  const [entries, setEntries] = useState<AuditLogEntry[]>([]);
  const [state, setState] = useState<ConnectionState>("connecting");
  const pausedRef = useRef(paused);
  pausedRef.current = paused;

  useEffect(() => {
    if (typeof window === "undefined" || !("EventSource" in window)) {
      return;
    }

    const url = filter ? `/api/stream/logs?${filter}` : "/api/stream/logs";
    const es = new EventSource(url);
    setState("connecting");

    es.onopen = () => setState("connected");
    es.onerror = () => setState("reconnecting");
    es.onmessage = (event) => {
      if (pausedRef.current) return;
      try {
        const parsed = JSON.parse(event.data) as AuditLogEntry;
        setEntries((prev) => {
          const next = [parsed, ...prev];
          return next.length > bufferSize ? next.slice(0, bufferSize) : next;
        });
      } catch {
        // ignore malformed events
      }
    };

    return () => {
      es.close();
    };
  }, [bufferSize, filter]);

  // Reflect external pause state in the indicator.
  useEffect(() => {
    if (paused) setState("paused");
  }, [paused]);

  return { entries, state };
}
