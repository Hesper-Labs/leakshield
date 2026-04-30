# API Reference

LeakShield exposes two surface areas:

1. **Admin REST API** under `/admin/v1/*`. JSON. Bearer-token auth (HS256 JWT issued by the
   gateway itself).
2. **Public proxy** under `/openai/v1/*`, `/anthropic/v1/*`, `/google/v1beta/*`,
   `/azure/openai/*`, `/v1/*`. Pass-through to upstream providers. Virtual key auth.

The full machine-readable contract is [`docs/openapi.yaml`](https://github.com/Hesper-Labs/leakshield/blob/main/docs/openapi.yaml). This page is the human-readable
overview.

## Admin REST

### Public (no auth)

```
GET  /admin/v1/setup/status
POST /admin/v1/auth/bootstrap
POST /admin/v1/auth/login
```

### Authenticated (`Authorization: Bearer <jwt>`)

```
GET    /admin/v1/me
POST   /admin/v1/auth/logout                # planned

GET    /admin/v1/providers
POST   /admin/v1/providers
POST   /admin/v1/providers/test
PATCH  /admin/v1/providers/{id}             # planned
DELETE /admin/v1/providers/{id}

GET    /admin/v1/users
POST   /admin/v1/users
GET    /admin/v1/users/{id}
PATCH  /admin/v1/users/{id}                 # planned
DELETE /admin/v1/users/{id}                 # planned
POST   /admin/v1/users/import               # planned

GET    /admin/v1/users/{id}/keys
POST   /admin/v1/users/{id}/keys
DELETE /admin/v1/keys/{id}
POST   /admin/v1/keys/{id}/rotate           # planned

GET    /admin/v1/policies                   # planned
POST   /admin/v1/policies                   # planned
PUT    /admin/v1/policies/{id}              # planned
GET    /admin/v1/policies/{id}/versions     # planned
POST   /admin/v1/policies/{id}/test         # planned

GET    /admin/v1/audit                      # planned
GET    /admin/v1/audit/{request_id}         # planned
GET    /admin/v1/analytics/usage            # planned
GET    /admin/v1/analytics/cost             # planned
GET    /admin/v1/analytics/blocks           # planned

GET    /admin/v1/stream/logs                # planned (SSE)
```

`# planned` endpoints are designed and documented in `docs/openapi.yaml` but not yet wired in
the Go gateway — the admin panel surfaces them and falls back to a demo banner when they 404.

## Authentication

The bootstrap endpoint creates the very first admin and returns a JWT in one shot:

```bash
curl -X POST http://localhost:8080/admin/v1/auth/bootstrap \
  -H 'Content-Type: application/json' \
  -d '{
    "company_name": "Acme",
    "full_name":    "Jane Q",
    "email":        "jane@acme.test",
    "password":     "correcthorsebattery"
  }'
```

Response (201):

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user":  { "id": "...", "email": "jane@acme.test", "tenant_id": "...", "role": "super_admin" }
}
```

Subsequent requests carry the token:

```bash
curl http://localhost:8080/admin/v1/me \
  -H "Authorization: Bearer <token>"
```

### Subsequent logins

```bash
curl -X POST http://localhost:8080/admin/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"jane@acme.test","password":"correcthorsebattery"}'
```

JWTs are HS256, valid for 24 hours. The signing secret is configured via
`LEAKSHIELD_JWT_SECRET` or auto-generated on first run into `~/.leakshield/jwt.secret`.

## End-to-end walkthrough

```bash
# 1. Bootstrap the first admin.
TOKEN=$(curl -sS -X POST http://localhost:8080/admin/v1/auth/bootstrap \
  -H 'Content-Type: application/json' \
  -d '{"company_name":"Acme","full_name":"Jane","email":"jane@acme.test","password":"correcthorsebattery"}' \
  | jq -r .token)

# 2. Connect a provider.
curl -sS -X POST http://localhost:8080/admin/v1/providers \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"provider":"openai","label":"Acme OpenAI","api_key":"sk-..."}'

# 3. Create a user and issue a virtual key.
USER_ID=$(curl -sS -X POST http://localhost:8080/admin/v1/users \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"Jane","email":"jane@acme.test"}' | jq -r .id)

KEY=$(curl -sS -X POST http://localhost:8080/admin/v1/users/$USER_ID/keys \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"laptop","allowed_providers":["openai","anthropic"]}' \
  | jq -r .plaintext)

# 4. Make a real request.
curl http://localhost:8080/openai/v1/chat/completions \
  -H "Authorization: Bearer $KEY" -H 'Content-Type: application/json' \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"Hello"}]}'
```

## Errors

| HTTP | Meaning |
|---|---|
| 400 | Validation error. Body `{ "error": "<message>" }`. |
| 401 | Missing or invalid bearer token (admin) or virtual key (proxy). |
| 403 | Forbidden — DLP block, allowlist denial, or insufficient role. Body includes `error` and (for DLP) `category` + `reason`. |
| 404 | Endpoint not implemented yet (see `# planned` list) or resource missing. The panel translates 404 from `/admin/v1/*` into "demo mode" banners so users keep their flow. |
| 409 | Conflict — e.g., bootstrap on an already-bootstrapped install. |
| 413 | Request body exceeds the 8 MiB cap. |
| 429 | Rate limit. `Retry-After` set. |
| 502 | Upstream provider 5xx. |
| 503 | Inspector or DB unreachable; respects `policy.fail_mode`. |

## Public proxy

The proxy paths are pass-through; their request and response bodies match the upstream provider
exactly. The only thing the gateway changes:

- `Authorization` / `x-api-key` / `api-key` swapped from your virtual key to the master key the
  gateway resolved.
- For Azure, the URL is rewritten to `{endpoint}/openai/deployments/{deployment}/chat/completions?api-version=...`.
- For Anthropic and OpenAI, on streaming responses the SSE bytes are forwarded unchanged.
- For Google, you can pass auth as `?key=` (gateway swaps it) or `Authorization: Bearer` (gateway
  preserves it and drops the query key).

The full details (and which provider features survive untouched) live in
[Architecture](Architecture#multi-protocol-native-gateway).
