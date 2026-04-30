# LeakShield API

LeakShield exposes two surfaces:

- An **admin REST API** rooted at `/admin/v1/*`, used by the panel and operator
  scripts to manage providers, users, virtual keys, DLP policies, and to read
  audit and analytics data.
- A **proxy API** that mirrors each upstream provider's native shape. Existing
  SDKs work by changing only the `base_url` and the API key.

The full machine-readable contract is [openapi.yaml](openapi.yaml). Generate a
client with the tool of your choice — the panel uses
[`openapi-typescript`](https://github.com/openapi-ts/openapi-typescript) plus
[`openapi-fetch`](https://github.com/openapi-ts/openapi-typescript/tree/main/packages/openapi-fetch);
Go consumers can use [`oapi-codegen`](https://github.com/oapi-codegen/oapi-codegen)
or hand-rolled clients.

## Authentication

| Surface | Header | Format |
|---|---|---|
| Admin REST | `Authorization: Bearer <session-token>` | JWT issued by `/admin/v1/auth/login` |
| Proxy (default) | `Authorization: Bearer gw_live_<...>` | Virtual key |
| Proxy (Anthropic) | `x-api-key: gw_live_<...>` | Anthropic native style |
| Proxy (Azure) | `api-key: gw_live_<...>` | Azure native style |

## Request flow on a fresh install

The first nine `curl` calls cover the entire onboarding, end-to-end. Each call
assumes you have set:

```sh
export LS=http://localhost:8080
```

### 1. Setup status

```sh
curl -sS "$LS/admin/v1/setup/status"
# { "completed": false, "version": "0.1.0", "kek_provider": "local", "inspector_backend": "mock" }
```

### 2. Bootstrap (only valid while `completed=false`)

```sh
curl -sS -X POST "$LS/admin/v1/auth/bootstrap" \
  -H 'content-type: application/json' \
  -d '{
    "company_name": "Acme Inc.",
    "company_slug": "acme",
    "admin_email": "admin@example.com",
    "admin_password": "long-strong-passphrase",
    "admin_name": "Admin User"
  }'
# Response: LoginResponse — keep the access_token.
```

### 3. Save the session token

```sh
export TOKEN=eyJhbGciOi...      # from the bootstrap or login response
```

### 4. Login (subsequent admin sessions)

```sh
curl -sS -X POST "$LS/admin/v1/auth/login" \
  -H 'content-type: application/json' \
  -d '{ "email": "admin@example.com", "password": "long-strong-passphrase" }'
```

### 5. Add a master provider key

```sh
curl -sS -X POST "$LS/admin/v1/providers" \
  -H "authorization: Bearer $TOKEN" \
  -H 'content-type: application/json' \
  -d '{
    "provider": "openai",
    "label": "OpenAI prod",
    "api_key": "sk-proj-..."
  }'
```

### 6. Create an end user

```sh
curl -sS -X POST "$LS/admin/v1/users" \
  -H "authorization: Bearer $TOKEN" \
  -H 'content-type: application/json' \
  -d '{
    "email": "ada@example.com",
    "name": "Ada",
    "surname": "Lovelace",
    "department": "Engineering"
  }'
# Response: User. Save the id as USER_ID.
```

### 7. Mint a virtual key for that user

```sh
curl -sS -X POST "$LS/admin/v1/users/$USER_ID/keys" \
  -H "authorization: Bearer $TOKEN" \
  -H 'content-type: application/json' \
  -d '{ "name": "ada-laptop", "allowed_providers": ["openai"] }'
# Response: VirtualKeyMinted — `api_key` is shown ONCE.
```

### 8. Make the first proxy request

```sh
export GW_KEY=gw_live_...

curl -sS -X POST "$LS/openai/v1/chat/completions" \
  -H "authorization: Bearer $GW_KEY" \
  -H 'content-type: application/json' \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{ "role": "user", "content": "Hello!" }]
  }'
```

If the default DLP policy is `mock`, the request is allowed straight through.
Switch to `hybrid` or `judge` once you have a model server running and re-run
to see DLP in action.

### 9. Read the audit log

```sh
curl -sS "$LS/admin/v1/audit?from=$(date -u -v-1H +%Y-%m-%dT%H:%M:%SZ)&to=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -H "authorization: Bearer $TOKEN"
```

## Streaming

Both the admin live-log endpoint (`/admin/v1/stream/logs`) and the upstream
provider streams (e.g. OpenAI `stream: true`, Anthropic `stream: true`) use
Server-Sent Events. The gateway preserves event boundaries byte-for-byte and
adds no buffering beyond what the provider sends.

Reconnect with `Last-Event-ID` to resume the admin stream without gaps.

## Errors

Every error response uses the same shape:

```json
{
  "error": {
    "code": "dlp_blocked",
    "message": "Request blocked by DLP policy."
  },
  "request_id": "8b3f5...e1"
}
```

`request_id` matches the `request_id` indexed on `audit_logs`, so operators can
look up exactly what happened.

## Versioning

The admin API is versioned in the URL prefix (`/admin/v1`). Breaking changes
ship under a new prefix; additive changes are made in place. The proxy API
versions follow the upstream provider — if OpenAI ships `/v2/...`, LeakShield
mirrors it.
