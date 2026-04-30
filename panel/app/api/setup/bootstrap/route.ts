import { NextResponse } from "next/server";

import { getInternalGatewayUrl } from "@/lib/env";

export const dynamic = "force-dynamic";

interface BootstrapBody {
  company_name?: string;
  full_name?: string;
  email?: string;
  password?: string;
}

/**
 * Proxies the create-admin form to the gateway's bootstrap endpoint.
 *
 * Returns:
 *   200 + { ok: true }                     — admin created
 *   400 + { ok: false, error: string }     — validation / business error
 *   503 + { ok: false, error, demo: true } — gateway not reachable; the panel
 *                                             stays in demo mode and the form
 *                                             surfaces a friendly diagnostic.
 */
export async function POST(req: Request) {
  let body: BootstrapBody;
  try {
    body = (await req.json()) as BootstrapBody;
  } catch {
    return NextResponse.json(
      { ok: false, error: "invalid request body" },
      { status: 400 },
    );
  }

  const errors = validate(body);
  if (errors.length > 0) {
    return NextResponse.json(
      { ok: false, error: errors.join("; ") },
      { status: 400 },
    );
  }

  const url = `${getInternalGatewayUrl().replace(/\/$/, "")}/admin/v1/auth/bootstrap`;
  const ctrl = new AbortController();
  const timer = setTimeout(() => ctrl.abort(), 5000);

  try {
    const res = await fetch(url, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
      cache: "no-store",
      signal: ctrl.signal,
    });

    if (res.status === 404) {
      return NextResponse.json(
        {
          ok: false,
          demo: true,
          error:
            "Gateway is up but the bootstrap endpoint is not implemented yet (Track D). The form is wired and ready.",
        },
        { status: 503 },
      );
    }

    if (!res.ok) {
      const text = await res.text();
      return NextResponse.json(
        { ok: false, error: text || `gateway returned ${res.status}` },
        { status: res.status },
      );
    }

    const data = await res.json();
    return NextResponse.json({ ok: true, ...data });
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

function validate(body: BootstrapBody): string[] {
  const errors: string[] = [];
  if (!body.company_name || body.company_name.trim().length < 2) {
    errors.push("company name is required");
  }
  if (!body.full_name || body.full_name.trim().length < 2) {
    errors.push("your name is required");
  }
  if (!body.email || !/^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(body.email)) {
    errors.push("a valid email is required");
  }
  if (!body.password || body.password.length < 8) {
    errors.push("password must be at least 8 characters");
  }
  return errors;
}
