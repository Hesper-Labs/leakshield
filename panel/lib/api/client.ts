import createClient, { type Client } from "openapi-fetch";

import { PUBLIC_API_URL, getInternalGatewayUrl } from "@/lib/env";

/**
 * The gateway publishes one OpenAPI spec at `/admin/v1/openapi.json`.
 * `scripts/generate-api-types.sh` runs `openapi-typescript` against that
 * spec and writes `lib/api/types.ts`. The generated file is gitignored
 * and must be regenerated locally; until it exists we fall back to the
 * loose `paths` interface defined here so the panel still typechecks.
 *
 * Once the gateway is wired up, replace the import below with:
 *
 *   import type { paths } from "@/lib/api/types";
 */
export interface paths {
  // intentionally empty — gets replaced by openapi-typescript output.
}

const isServer = typeof window === "undefined";

export function makeApiClient(opts?: {
  token?: string;
  baseUrl?: string;
}): Client<paths> {
  const baseUrl = opts?.baseUrl ?? (isServer ? getInternalGatewayUrl() : PUBLIC_API_URL);
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };
  if (opts?.token) {
    headers["Authorization"] = `Bearer ${opts.token}`;
  }
  return createClient<paths>({ baseUrl, headers });
}

/**
 * Browser-side singleton client. Server-side callers should use
 * `makeApiClient({ token })` so each request gets the caller's session
 * token rather than a shared instance.
 */
export const browserApi = isServer
  ? null
  : (createClient<paths>({ baseUrl: PUBLIC_API_URL }) as Client<paths>);
