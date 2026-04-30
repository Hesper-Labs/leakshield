import { NextResponse } from "next/server";

import { proxyToGateway, readJsonBody } from "@/lib/admin-proxy";

export const dynamic = "force-dynamic";

interface TestProviderBody {
  provider?: string;
  api_key?: string;
  config?: Record<string, unknown>;
}

/**
 * Proxies the "Test connection" button on onboarding step 2 to
 * `POST /admin/v1/providers/test`. The gateway is responsible for
 * making a cheap upstream call (e.g. `GET /v1/models` for OpenAI) and
 * returning the discovered model list.
 *
 * Expected gateway response shape:
 *   200 { ok: true,  models: string[] }
 *   4xx { ok: false, error: string }
 */
export async function POST(req: Request) {
  const body = await readJsonBody<TestProviderBody>(req);
  if ("__error" in body) {
    return NextResponse.json({ ok: false, error: body.__error }, { status: 400 });
  }

  if (!body.provider || typeof body.provider !== "string") {
    return NextResponse.json(
      { ok: false, error: "provider is required" },
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
    path: "/admin/v1/providers/test",
    method: "POST",
    body: {
      provider: body.provider,
      api_key: body.api_key,
      config: body.config ?? {},
    },
    // Upstream model-list probes can be slow on cold paths.
    timeoutMs: 12000,
  });
}
