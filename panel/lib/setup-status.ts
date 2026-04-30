import { getInternalGatewayUrl } from "@/lib/env";

/**
 * Result of probing the gateway to learn whether the install is fresh.
 *
 * The panel uses this to decide where the root URL should send a visitor:
 *   - reachable + !hasAdmin → /onboarding/1 (create the very first admin)
 *   - reachable + hasAdmin  → /sign-in
 *   - !reachable            → /onboarding/1 in demo mode, with a banner
 *
 * The third case keeps the panel browsable when the gateway is offline.
 * That matters for OSS adoption: contributors should be able to look at the
 * UI without bringing up Postgres + Redis + Go binaries first.
 */
export type SetupStatus =
  | {
      reachable: true;
      hasAdmin: boolean;
      acceptsBootstrap: boolean;
    }
  | {
      reachable: false;
      reason: string;
    };

const PROBE_TIMEOUT_MS = 1500;

export async function fetchSetupStatus(): Promise<SetupStatus> {
  const url = `${getInternalGatewayUrl().replace(/\/$/, "")}/admin/v1/setup/status`;
  const ctrl = new AbortController();
  const timer = setTimeout(() => ctrl.abort(), PROBE_TIMEOUT_MS);

  try {
    const res = await fetch(url, {
      method: "GET",
      headers: { Accept: "application/json" },
      cache: "no-store",
      signal: ctrl.signal,
    });

    if (!res.ok) {
      // The endpoint is part of Track D and may not exist yet. Treat any
      // non-success response as "fresh install" so onboarding still flows.
      if (res.status === 404) {
        return { reachable: true, hasAdmin: false, acceptsBootstrap: true };
      }
      return { reachable: false, reason: `gateway returned ${res.status}` };
    }

    const data = (await res.json()) as Partial<{
      has_admin: boolean;
      accepts_bootstrap: boolean;
    }>;
    return {
      reachable: true,
      hasAdmin: Boolean(data.has_admin),
      acceptsBootstrap: data.accepts_bootstrap !== false,
    };
  } catch (e) {
    return { reachable: false, reason: e instanceof Error ? e.message : String(e) };
  } finally {
    clearTimeout(timer);
  }
}
