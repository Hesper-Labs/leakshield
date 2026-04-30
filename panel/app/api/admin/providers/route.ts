import { NextResponse } from "next/server";

import { proxyToGateway, readJsonBody } from "@/lib/admin-proxy";

export const dynamic = "force-dynamic";

interface SaveProviderBody {
  provider?: string;
  label?: string;
  api_key?: string;
  config?: Record<string, unknown>;
  allowed_models?: string[];
}

/**
 * GET  -> list saved providers      (`GET  /admin/v1/providers`)
 * POST -> persist a new provider    (`POST /admin/v1/providers`)
 *
 * The save call mirrors the "Test connection" body, plus a human
 * label and the optional `allowed_models` whitelist that the gateway
 * uses for routing.
 */
export async function GET() {
  return proxyToGateway({
    path: "/admin/v1/providers",
    method: "GET",
  });
}

export async function POST(req: Request) {
  const body = await readJsonBody<SaveProviderBody>(req);
  if ("__error" in body) {
    return NextResponse.json({ ok: false, error: body.__error }, { status: 400 });
  }

  if (!body.provider || typeof body.provider !== "string") {
    return NextResponse.json(
      { ok: false, error: "provider is required" },
      { status: 400 },
    );
  }
  if (!body.label || body.label.trim().length === 0) {
    return NextResponse.json(
      { ok: false, error: "label is required" },
      { status: 400 },
    );
  }
  if (!body.api_key || typeof body.api_key !== "string") {
    return NextResponse.json(
      { ok: false, error: "api_key is required" },
      { status: 400 },
    );
  }

  return proxyToGateway({
    path: "/admin/v1/providers",
    method: "POST",
    body: {
      provider: body.provider,
      label: body.label.trim(),
      api_key: body.api_key,
      config: body.config ?? {},
      allowed_models: body.allowed_models ?? [],
    },
  });
}
