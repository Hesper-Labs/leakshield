import { proxyToGateway } from "@/lib/admin-proxy";

export const dynamic = "force-dynamic";

/**
 * Returns the currently signed-in admin and their tenant — used by step 3
 * to pre-fill the "Use my account" tab so the operator doesn't have to
 * re-type their own name.
 *
 * Expected gateway response shape:
 *   { user: { id, name, email, role }, tenant: { id, name, slug } }
 */
export async function GET() {
  return proxyToGateway({
    path: "/admin/v1/me",
    method: "GET",
  });
}
