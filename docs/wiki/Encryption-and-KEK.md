# Encryption and KEK

LeakShield's master credentials — the company's own `sk-...` keys for OpenAI, Anthropic, Google,
Azure — are stored encrypted at rest. The encryption is layered:

```
KEK (32 bytes)
 ↓ wraps
DEK per company (32 bytes)
 ↓ encrypts (AES-256-GCM)
master provider keys (sk-…) + optional full-prompt audit log entries
```

The Key Encryption Key never lives in the database. Only the wrapped Data Encryption Key does
(`companies.dek_wrapped`).

## KEK providers

LeakShield ships with one production-ready provider (`local`) and stubs for the major KMS
options. The choice lives in `LEAKSHIELD_KMS_PROVIDER`:

| Provider | Status | Notes |
|---|---|---|
| `local` | shipped | reads a 32-byte file at `LEAKSHIELD_KEK_FILE` (or `~/.leakshield/kek`). Refused under `--prod` unless an explicit path is set. |
| `vault` | stub | HashiCorp Vault Transit (TODO) |
| `aws` | stub | AWS KMS GenerateDataKey / Decrypt (TODO) |
| `gcp` | stub | GCP KMS (TODO) |
| `azure` | stub | Azure Key Vault (TODO) |

The stubs return a clear "not yet wired" error pointing at the follow-up tracker; bring-up of any
of them is on the [Roadmap](Roadmap).

## Local KEK (development default)

On first run, if no KEK file exists, the gateway:

1. Creates `~/.leakshield/kek` with mode 0600.
2. Writes 32 random bytes from `crypto/rand`.
3. Logs a WARN telling the operator to back the file up.

If the file exists but has insecure permissions (anything beyond 0600), the gateway refuses to
start. Set yours back with `chmod 0600 ~/.leakshield/kek`.

In `--prod` mode (`LEAKSHIELD_PROD=1`), local KEK auto-generation is disabled. Either point at
a pre-provisioned file or pick a KMS provider.

## DEK lifecycle

When a tenant is bootstrapped:

```go
dek, _ := crypto.GenerateDEK()                      // 32 bytes from crypto/rand
wrappedDEK, _ := kek.WrapDEK(dek)                   // AES-256-GCM under the KEK
INSERT INTO companies (..., dek_wrapped, kek_id)    // dek_wrapped persisted
```

When a request needs the tenant's plaintext provider key:

```go
c, _ := db.FindCompanyByID(tenantID)
dek, _ := kek.UnwrapDEK(c.DEKWrapped)               // AES-256-GCM open
mk, _ := db.FindActiveMasterKey(tenantID, "openai")
plaintext, _ := crypto.DecryptWithDEK(dek, ciphertext)
```

The unwrapped DEK is held in an in-memory cache keyed by tenant ID, with a 15-minute TTL to bound
damage on memory disclosure. The cache is per-process; restart drops it.

`Resolver.Invalidate(tenantID)` clears the entry. Callers should use it after key rotation or
revocation.

## Master provider keys at rest

`master_provider_keys.api_key_cipher` and `master_provider_keys.api_key_nonce` together form
the AES-256-GCM ciphertext + 12-byte nonce. The `Resolver`'s convenience layer hides the layout:

```go
cipher, nonce, _ := resolver.EncryptForTenant(ctx, tenantID, []byte("sk-proj-xxxx"))
INSERT INTO master_provider_keys (api_key_cipher, api_key_nonce, ...)

// later:
plain, mk, _ := resolver.MasterKey(ctx, tenantID, "openai")
```

The `config` JSONB column carries provider-specific extras (Azure endpoint + deployment map,
OpenAI org id, etc.) — those aren't sensitive and stay in plaintext.

## Audit log prompt encryption (optional)

By default the audit log stores `prompt_hash` (SHA-256) and `prompt_preview` (first 80 chars +
"…" + last 80 chars, with PII spans masked already). Setting
`dlp_policies.audit_full_prompt = true` switches a tenant to also store
`prompt_encrypted` + `prompt_nonce` — AES-256-GCM under the tenant DEK.

This is an explicit, per-tenant opt-in because storing full prompts (even encrypted) is a
compliance choice with material consequences (right-to-erasure scope, breach blast radius).

## Rotation

### KEK rotation

Re-wrap every tenant's DEK under the new KEK, write the new wrapped value, atomically swap. Data
plane untouched (master keys + audit prompts are encrypted with the DEK, not the KEK).

### DEK rotation

Per tenant: generate a new DEK, re-encrypt every `master_provider_keys` ciphertext (and audit
prompts if used) under it, swap the `companies.dek_wrapped` value. The DEK cache is invalidated
once the swap commits.

### Provider key rotation

Per provider per tenant: insert a new `master_provider_keys` row referencing the previous via
`rotated_from_id`. The old key gets `is_active = false` after a 24-hour grace period, during
which in-flight requests using the cached old key still succeed.

## Threats and mitigations

| Threat | Mitigation |
|---|---|
| KEK file leak (laptop theft, backup misconfiguration) | KMS-backed KEK in production. Local KEK is dev-only and the gateway warns loudly. |
| Postgres breach | DEK is stored *wrapped* by the KEK. Without the KEK, the attacker has only `argon2id` hashes and AES-256-GCM ciphertexts. Master keys + audit prompts are unrecoverable. |
| Process memory disclosure | DEK cache TTL bounds exposure (default 15 min). Cache is dropped on process restart. KEK is read once at startup; consider sealing it after read. |
| KMS-side compromise | Vault / KMS access logs reveal usage; rate-limit DEK-unwrap operations and alert on anomalies. The KMS-backed providers (when shipped) all expose access tracing. |
| GCM nonce reuse | Each encrypt call generates a fresh 12-byte nonce from `crypto/rand`. The probability of collision over the encrypted-objects-per-tenant horizon (millions) is negligible. |
| Side-channel timing | All comparisons (argon2id verify, key prefix lookup) use constant-time primitives from `crypto/subtle` or the argon2 library. |

## Operator runbook

- **Backups**: back up `~/.leakshield/kek` (or the KMS key) **separately** from the database.
  Anyone with the DB but not the KEK has nothing useful; anyone with both has everything. Treat
  the KEK like you'd treat a database master password.
- **Rotation cadence**: rotate provider keys per the upstream provider's recommendation
  (typically 90 days). Rotate DEKs annually or on staff turnover. Rotate KEK on suspected
  compromise.
- **Incident response**: if the KEK is suspected compromised, rotate immediately. The data plane
  ciphertexts are still safe (they're under the DEKs, not the KEK directly), but a new KEK +
  rewrap of all DEKs closes the window.
