# Virtual Keys

Every employee that uses LeakShield carries a *virtual key*. The gateway issues virtual keys;
they have a known prefix that's safe to log, and a 32-character secret that's hashed at rest with
argon2id. Virtual keys never grant access to the company's actual provider credentials — when a
client presents one the gateway transparently swaps it for the appropriate master key.

## Format

```
gw_<env>_<8-char-prefix>_<32-char-secret>
   └─┬─┘  └──────┬─────┘  └────────┬───────┘
   live | test  lookup           secret
```

| Segment | Role | Length |
|---|---|---|
| `gw_` | constant scheme tag | 3 |
| `<env>` | `live` or `test` (separates production keys from sandbox testing) | 4 |
| `<8-char-prefix>` | random `[a-z0-9]`; used for O(1) DB lookup | 8 |
| `<32-char-secret>` | URL-safe base64 random; verified via argon2id | 32 |

The first three segments together (`gw_live_<prefix>`) form what we call the **lookup prefix**.
That's what's stored in `virtual_keys.key_prefix` (UNIQUE), and that's what shows up in the
panel's key list, the audit log, and any CLI debug output.

The full plaintext secret is shown to the admin **once** at the moment of creation. It's never
re-served. Lose it → revoke the key and issue a new one.

## Lifecycle

```
                 ┌─ generate (panel: User → Generate key) ─┐
                 │                                          │
                 ▼                                          │
            virtual_keys row                                │
                 │                                          │
                 ▼                                          │
   client uses key ──────► gateway extractPresentedKey ──► verify (LRU + argon2id)
                                                           │
                                                           ▼
                                                       allowedProviders + tenantId attached
                                                           │
   ┌───────────────────────────────────────────────────────┤
   │                                                       │
   ▼                                                       ▼
   revoke (panel)                                          rotate (panel)
   sets revoked_at = now()                                 issues new key, marks old expires_at
                                                           in 7 days for zero-downtime cutover
```

## Storage

The `virtual_keys` row holds:

- `id` UUID
- `company_id` (RLS scope)
- `user_id` (the employee this key belongs to)
- `name` (admin-set label, e.g. "claude-code laptop")
- `key_prefix` (e.g. `gw_live_a1b2c3d4`) — UNIQUE, indexed
- `key_hash` (argon2id of the 32-char secret; m=64 MiB, t=3, p=2, 32-byte output)
- `allowed_providers` TEXT[]
- `allowed_models` TEXT[]
- `rpm_limit`, `tpm_limit`, `monthly_token_limit`, `monthly_usd_micro_limit`
- `expires_at`, `revoked_at`, `last_used_at`, `created_at`

`is_active` is computed at read time:

```go
func (vk *VirtualKey) IsActive() bool {
    if vk.RevokedAt != nil { return false }
    if vk.ExpiresAt != nil && time.Now().After(*vk.ExpiresAt) { return false }
    return true
}
```

## Verification

Per request the gateway:

1. Pulls the key from `Authorization: Bearer …`, `x-api-key:`, `api-key:`, or `?key=` (every
   provider's SDK uses something different and we accept all of them).
2. Splits into `<scheme>_<env>_<prefix>_<secret>`. Rejects malformed inputs with 401.
3. Checks the verifier's in-memory cache (60 s positive TTL, 5 s negative TTL). On cache hit it
   confirms with a fast non-cryptographic hash that the same secret produced the cached entry —
   different secrets with the same prefix would be a brute-force probe and never share a cache.
4. On cache miss: `SELECT … FROM virtual_keys WHERE key_prefix = $1 AND revoked_at IS NULL`,
   then `argon2.IDKey(secret, salt, …)` constant-time verify against `key_hash`.
5. Attaches a `VirtualKeyContext{KeyID, TenantID, UserID, AllowedProviders, AllowedModels}` to
   the request context. Subsequent middleware enforces the allowlist; the chat handler resolves
   the master provider key from the tenant DEK.

The fast non-cryptographic per-secret hash is `uintHex(FNV-style)` and is **not** used for
authentication — only for cache safety. The argon2id verify still happens on first hit.

## Distribution to clients

The plaintext key looks like every other API key today, so existing SDKs and CLIs accept it
without modification:

```bash
# Claude Code CLI (Anthropic native)
export ANTHROPIC_BASE_URL=http://leakshield.example.com/anthropic
export ANTHROPIC_API_KEY=gw_live_a1b2c3d4_xK9pZmL...
claude
```

```python
# OpenAI Python SDK
from openai import OpenAI
client = OpenAI(
    base_url="http://leakshield.example.com/openai/v1",
    api_key="gw_live_a1b2c3d4_xK9pZmL...",
)
```

```python
# Google Gemini
import google.generativeai as genai
genai.configure(
    api_key="gw_live_a1b2c3d4_xK9pZmL...",
    transport="rest",
    client_options={"api_endpoint": "http://leakshield.example.com/google"},
)
```

See [Client Examples](Client-Examples) for the full set of SDKs / CLIs we've validated.

## Per-key allowlists and budgets

The panel surfaces:

- **`allowed_providers`** — empty array means "allow any active provider"; otherwise the gateway
  rejects requests for providers not in the list.
- **`allowed_models`** — same idea, scoped to model names.
- **`rpm_limit`** / **`tpm_limit`** — Redis sliding window, per minute. 0 / null disables.
- **`monthly_token_limit`** / **`monthly_usd_micro_limit`** — soft cap; the worker tallies
  `usage_aggregates` and returns 429 when exceeded. Re-evaluated daily so you can set a `$50/mo`
  laptop budget for a junior dev.

## Revocation + rotation

- **Revoke**: panel button or `DELETE /admin/v1/keys/{id}`. Sets `revoked_at = now()`. The
  verifier's in-memory cache is invalidated via Redis pub-sub so all gateway nodes drop the
  cached row immediately. Next request from the revoked key returns 401.
- **Rotate**: panel button (TODO in v1) or `POST /admin/v1/keys/{id}/rotate`. Issues a new key,
  sets the old key's `expires_at` to 7 days from now. SDKs / CLIs that support a fallback list
  (`keys: [primary, fallback]`) get zero-downtime cutover.

## Storing the plaintext (don't)

The plaintext is shown once. The DB never stores it. The panel never stores it. The verifier
cache holds a digest of it for 60 s and then drops it. If a user loses their key, they revoke
and rotate — there is no recovery path. This is a feature, not a bug.

## Audit visibility

Every request the gateway proxies records:

- `virtual_keys.last_used_at` (best-effort, async update)
- An `audit_logs` row with `virtual_key_id`, `provider`, `model`, `status`, token counts, latency,
  cost, IP, user-agent, prompt hash, and (when `policy.audit_full_prompt = true`) the full
  prompt encrypted under the tenant DEK.

The panel's Logs page surfaces this in real time over SSE.
