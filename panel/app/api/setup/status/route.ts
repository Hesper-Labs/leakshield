import { NextResponse } from "next/server";

import { fetchSetupStatus } from "@/lib/setup-status";

export const dynamic = "force-dynamic";

export async function GET() {
  const status = await fetchSetupStatus();
  return NextResponse.json(status);
}
