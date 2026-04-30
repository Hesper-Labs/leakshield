"use client";

import Link from "next/link";
import { useEffect, useMemo, useRef, useState } from "react";

import { Badge } from "@/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  ConnectionIndicator,
  type ConnectionState,
} from "@/components/layout/connection-indicator";
import {
  type AuditLogEntry,
  useLogStream,
} from "@/hooks/use-log-stream";

/**
 * Live audit log table. Entries flow in from the SSE proxy at the top.
 * Auto-pause kicks in as soon as the user scrolls below the first row
 * — preventing flicker while they read — and resumes when they scroll
 * back to the top.
 *
 * Virtualization is sketched here as a windowed render of the most
 * recent 200 rows. We can swap to `@tanstack/react-virtual` in a
 * follow-up without changing the call site.
 */
export function LiveLogTable() {
  const containerRef = useRef<HTMLDivElement>(null);
  const [paused, setPaused] = useState(false);
  const { entries, state } = useLogStream({ paused, bufferSize: 1000 });

  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    function onScroll() {
      if (!el) return;
      setPaused(el.scrollTop > 16);
    }
    el.addEventListener("scroll", onScroll, { passive: true });
    return () => el.removeEventListener("scroll", onScroll);
  }, []);

  const visible = useMemo(() => entries.slice(0, 200), [entries]);
  const indicatorState: ConnectionState = paused ? "paused" : state;

  return (
    <div className="overflow-hidden rounded-lg border bg-card">
      <div className="flex items-center justify-between border-b px-3 py-2">
        <ConnectionIndicator state={indicatorState} />
        <span className="text-xs text-muted-foreground">
          {entries.length} entries buffered
          {paused ? " · paused on scroll" : ""}
        </span>
      </div>
      <div ref={containerRef} className="max-h-[560px] overflow-y-auto">
        <Table>
          <TableHeader className="sticky top-0 bg-background">
            <TableRow>
              <TableHead className="w-[160px]">Time</TableHead>
              <TableHead>User</TableHead>
              <TableHead>Provider · Model</TableHead>
              <TableHead className="w-[110px]">Verdict</TableHead>
              <TableHead className="w-[100px]">Latency</TableHead>
              <TableHead>Category</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {visible.length === 0 ? (
              <TableRow>
                <TableCell colSpan={6} className="py-12 text-center text-sm text-muted-foreground">
                  Waiting for the first request from the gateway…
                </TableCell>
              </TableRow>
            ) : (
              visible.map((entry) => <LogRow key={entry.id} entry={entry} />)
            )}
          </TableBody>
        </Table>
      </div>
    </div>
  );
}

function LogRow({ entry }: { entry: AuditLogEntry }) {
  return (
    <TableRow>
      <TableCell className="font-mono text-xs">
        <Link href={`/logs/${entry.id}`} className="hover:underline">
          {new Date(entry.timestamp).toLocaleTimeString()}
        </Link>
      </TableCell>
      <TableCell className="text-xs">{entry.user}</TableCell>
      <TableCell className="text-xs">
        {entry.provider} · {entry.model}
      </TableCell>
      <TableCell>
        <VerdictBadge verdict={entry.verdict} />
      </TableCell>
      <TableCell className="text-xs">
        {entry.latencyMs ? `${entry.latencyMs} ms` : "—"}
      </TableCell>
      <TableCell className="text-xs text-muted-foreground">
        {entry.category ?? "—"}
      </TableCell>
    </TableRow>
  );
}

function VerdictBadge({
  verdict,
}: {
  verdict: AuditLogEntry["verdict"];
}) {
  switch (verdict) {
    case "ALLOW":
      return <Badge variant="success">{verdict}</Badge>;
    case "MASK":
      return <Badge variant="warning">{verdict}</Badge>;
    case "BLOCK":
      return <Badge variant="destructive">{verdict}</Badge>;
    default:
      return <Badge variant="outline">{verdict}</Badge>;
  }
}
