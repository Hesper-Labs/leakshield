/**
 * Centralized environment variable reading.
 *
 * The panel runs in two contexts:
 *   1. Browser code can only read NEXT_PUBLIC_* vars.
 *   2. Server code (RSC, Route Handlers, Auth.js) can read everything,
 *      and prefers in-network DNS for the gateway.
 */

export const PUBLIC_API_URL =
  process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export function getInternalGatewayUrl(): string {
  return process.env.GATEWAY_INTERNAL_URL ?? PUBLIC_API_URL;
}

export const DEFAULT_LOCALE =
  process.env.NEXT_PUBLIC_DEFAULT_LOCALE ?? "en";
