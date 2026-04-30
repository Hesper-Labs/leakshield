import { NextResponse } from "next/server";

import { proxyToGateway, readJsonBody } from "@/lib/admin-proxy";

export const dynamic = "force-dynamic";

interface CreateKeyBody {
  name?: string;
  allowed_providers?: string[];
  allowed_models?: string[];
  monthly_token_limit?: number;
  monthly_usd_micro_limit?: number;
}

/**
 * Issues a new virtual key for the user identified by `params.id`.
 * The gateway responds with the **plaintext** key exactly once — step 3's
 * UI shows it, lets the operator copy it, and never displays it again.
 *
 * Expected gateway response shape:
 *   { id, prefix, plaintext, allowed_providers?, allowed_models? }
 */
export async function POST(
  req: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const { id } = await params;
  if (!id || id.trim().length === 0) {
    return NextResponse.json(
      { ok: false, error: "user id is required" },
      { status: 400 },
    );
  }

  const body = await readJsonBody<CreateKeyBody>(req);
  if ("__error" in body) {
    return NextResponse.json({ ok: false, error: body.__error }, { status: 400 });
  }

  const payload: Record<string, unknown> = {
    name: body.name?.trim() && body.name.trim().length > 0 ? body.name.trim() : "Default key",
  };
  if (body.allowed_providers?.length) payload.allowed_providers = body.allowed_providers;
  if (body.allowed_models?.length) payload.allowed_models = body.allowed_models;
  if (typeof body.monthly_token_limit === "number") {
    payload.monthly_token_limit = body.monthly_token_limit;
  }
  if (typeof body.monthly_usd_micro_limit === "number") {
    payload.monthly_usd_micro_limit = body.monthly_usd_micro_limit;
  }

  return proxyToGateway({
    path: `/admin/v1/users/${encodeURIComponent(id)}/keys`,
    method: "POST",
    body: payload,
  });
}
