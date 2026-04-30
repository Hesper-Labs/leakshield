import { NextResponse } from "next/server";

import { proxyToGateway, readJsonBody } from "@/lib/admin-proxy";

export const dynamic = "force-dynamic";

interface CreateUserBody {
  name?: string;
  surname?: string;
  email?: string;
  phone?: string;
  department?: string;
  role?: string;
}

/**
 * GET  -> list tenant users         (`GET  /admin/v1/users`)
 * POST -> create a new tenant user  (`POST /admin/v1/users`)
 *
 * The created user is returned with `id`, which step 3 then uses to issue
 * a virtual key via `POST /admin/v1/users/{id}/keys`.
 */
export async function GET() {
  return proxyToGateway({
    path: "/admin/v1/users",
    method: "GET",
  });
}

export async function POST(req: Request) {
  const body = await readJsonBody<CreateUserBody>(req);
  if ("__error" in body) {
    return NextResponse.json({ ok: false, error: body.__error }, { status: 400 });
  }

  if (!body.name || body.name.trim().length < 2) {
    return NextResponse.json(
      { ok: false, error: "name is required" },
      { status: 400 },
    );
  }
  if (!body.email || !/^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(body.email)) {
    return NextResponse.json(
      { ok: false, error: "a valid email is required" },
      { status: 400 },
    );
  }

  const payload: Record<string, unknown> = {
    name: body.name.trim(),
    email: body.email.trim().toLowerCase(),
  };
  if (body.surname?.trim()) payload.surname = body.surname.trim();
  if (body.phone?.trim()) payload.phone = body.phone.trim();
  if (body.department?.trim()) payload.department = body.department.trim();
  if (body.role) payload.role = body.role;

  return proxyToGateway({
    path: "/admin/v1/users",
    method: "POST",
    body: payload,
  });
}
