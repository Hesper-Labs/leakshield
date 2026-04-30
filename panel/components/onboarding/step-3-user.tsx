import { auth } from "@/auth";
import { getInternalGatewayUrl } from "@/lib/env";

import { Step3UserClient, type MeData } from "./step-3-user-client";

/**
 * Server-side wrapper for onboarding step 3.
 *
 * Pulls the current admin's identity (and tenant) from the gateway so the
 * "Use my account" tab can be pre-filled without a client round-trip on
 * first paint. If the gateway is offline or doesn't implement `/admin/v1/me`
 * yet, we render the client with `me=null` and let it fall back to a
 * read-only "your account" pane that uses whatever Auth.js already has on
 * the session.
 */
export async function Step3User() {
  const session = await auth();
  const token = session?.accessToken;

  let me: MeData | null = null;

  if (token) {
    try {
      const url = `${getInternalGatewayUrl().replace(/\/$/, "")}/admin/v1/me`;
      const ctrl = new AbortController();
      const timer = setTimeout(() => ctrl.abort(), 2500);
      try {
        const res = await fetch(url, {
          method: "GET",
          headers: {
            Accept: "application/json",
            Authorization: `Bearer ${token}`,
          },
          cache: "no-store",
          signal: ctrl.signal,
        });
        if (res.ok) {
          const data = (await res.json()) as Partial<MeData>;
          if (data && data.user && data.user.id) {
            me = {
              user: {
                id: String(data.user.id),
                email: String(data.user.email ?? session.user.email ?? ""),
                name: String(data.user.name ?? session.user.name ?? ""),
                role: data.user.role,
              },
              tenant: data.tenant
                ? {
                    id: String(data.tenant.id ?? ""),
                    name: String(data.tenant.name ?? ""),
                    slug: data.tenant.slug ? String(data.tenant.slug) : undefined,
                  }
                : null,
            };
          }
        }
      } finally {
        clearTimeout(timer);
      }
    } catch {
      // Swallow — the client component shows a friendly demo-mode hint.
    }
  }

  // Final fallback: build a `me` from the Auth.js session payload alone.
  // The id is unavailable, so the "Use my account" tab will surface a
  // demo-mode notice instead of issuing a real key.
  const fallback: MeData | null =
    me ??
    (session?.user?.email
      ? {
          user: {
            id: "",
            email: session.user.email,
            name: session.user.name ?? session.user.email,
            role: session.user.role,
          },
          tenant: null,
        }
      : null);

  return <Step3UserClient me={fallback} />;
}
