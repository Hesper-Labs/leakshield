import { NextResponse } from "next/server";

import { auth } from "@/auth";
import { getInternalGatewayUrl } from "@/lib/env";

/**
 * Shared helper for the panel's `/api/admin/*` proxy routes.
 *
 * Each route is a tiny pass-through to the Go gateway that
 *   1. pulls the bearer token from the Auth.js session,
 *   2. forwards the request,
 *   3. translates the gateway's responses into one of three shapes the
 *      onboarding UI knows how to render:
 *
 *        - 200/2xx + payload                        — happy path
 *        - 4xx     + { ok:false, error }            — validation / business
 *        - 503     + { ok:false, demo:true, error } — gateway unreachable or
 *                                                     not implemented yet,
 *                                                     same shape as the
 *                                                     bootstrap route uses.
 *
 * The third case keeps the UI walkable when the gateway is offline, which
 * matters for OSS adoption — contributors should be able to click through
 * onboarding without spinning up Postgres + Redis + Go binaries.
 */

export interface ProxyOptions {
  /** Gateway path, with leading slash, e.g. `/admin/v1/providers`. */
  path: string;
  /** HTTP method to forward. */
  method: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
  /** Optional JSON body to forward. */
  body?: unknown;
  /** Per-call timeout. Defaults to 8 s — matches `bootstrap` plus headroom for
   *  upstream provider probes. */
  timeoutMs?: number;
}

const DEFAULT_TIMEOUT_MS = 8000;

export async function proxyToGateway(opts: ProxyOptions): Promise<Response> {
  const session = await auth();
  const token = session?.accessToken;
  if (!token) {
    return NextResponse.json(
      { ok: false, error: "not authenticated" },
      { status: 401 },
    );
  }

  const url = `${getInternalGatewayUrl().replace(/\/$/, "")}${opts.path}`;
  const ctrl = new AbortController();
  const timer = setTimeout(() => ctrl.abort(), opts.timeoutMs ?? DEFAULT_TIMEOUT_MS);

  try {
    const init: RequestInit = {
      method: opts.method,
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
        Authorization: `Bearer ${token}`,
      },
      cache: "no-store",
      signal: ctrl.signal,
    };
    if (opts.body !== undefined) {
      init.body = JSON.stringify(opts.body);
    }

    const res = await fetch(url, init);

    if (res.status === 404) {
      return NextResponse.json(
        {
          ok: false,
          demo: true,
          error: `Gateway is up but ${opts.path} is not implemented yet. The form is wired and ready.`,
        },
        { status: 503 },
      );
    }

    // Pass the upstream status + body straight through. Most gateway
    // endpoints already return JSON; if they return text we still want the
    // client to see the raw payload for diagnostics.
    const contentType = res.headers.get("content-type") ?? "";
    if (contentType.includes("application/json")) {
      const data = await res.json().catch(() => ({}));
      return NextResponse.json(data, { status: res.status });
    }

    const text = await res.text();
    return NextResponse.json(
      { ok: res.ok, error: res.ok ? undefined : text || `gateway returned ${res.status}` },
      { status: res.status },
    );
  } catch (e) {
    const reason = e instanceof Error ? e.message : String(e);
    return NextResponse.json(
      {
        ok: false,
        demo: true,
        error: `Gateway is not reachable (${reason}). Start it with: docker compose up gateway`,
      },
      { status: 503 },
    );
  } finally {
    clearTimeout(timer);
  }
}

/** Reads the JSON body from a route-handler `Request` with a generic guard. */
export async function readJsonBody<T>(req: Request): Promise<T | { __error: string }> {
  try {
    return (await req.json()) as T;
  } catch {
    return { __error: "invalid request body" };
  }
}
