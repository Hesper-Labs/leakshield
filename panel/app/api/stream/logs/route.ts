import { NextResponse } from "next/server";

import { auth } from "@/auth";
import { getInternalGatewayUrl } from "@/lib/env";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

/**
 * SSE proxy for the live audit log.
 *
 * Browsers can't easily attach `Authorization` headers to native
 * EventSource streams, so the panel proxies the upstream
 * `/admin/v1/stream/logs` SSE feed through this route. We resolve the
 * caller's session, attach the bearer token server-side, and stream
 * the upstream response straight back to the client.
 */
export async function GET(request: Request) {
  const session = await auth();
  if (!session?.accessToken) {
    return NextResponse.json({ error: "unauthenticated" }, { status: 401 });
  }

  const url = new URL(request.url);
  const upstreamUrl = new URL(
    "/admin/v1/stream/logs",
    getInternalGatewayUrl(),
  );
  // Forward filter query params (?virtual_key_id=, ?since=, etc.) verbatim.
  url.searchParams.forEach((value, key) => {
    upstreamUrl.searchParams.set(key, value);
  });

  const upstream = await fetch(upstreamUrl, {
    method: "GET",
    headers: {
      Accept: "text/event-stream",
      Authorization: `Bearer ${session.accessToken}`,
    },
    cache: "no-store",
    signal: request.signal,
  });

  if (!upstream.ok || !upstream.body) {
    return NextResponse.json(
      { error: "upstream_unavailable", status: upstream.status },
      { status: 502 },
    );
  }

  return new Response(upstream.body, {
    status: 200,
    headers: {
      "Content-Type": "text/event-stream",
      "Cache-Control": "no-cache, no-transform",
      Connection: "keep-alive",
      "X-Accel-Buffering": "no",
    },
  });
}
