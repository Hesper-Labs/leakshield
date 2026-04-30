# Security Policy

## Supported versions

LeakShield is pre-alpha. Only the `main` branch is currently supported.

## Reporting a vulnerability

If you find a security issue:

1. **Do not open a public GitHub issue.** Public issues alert attackers before a fix is in place.
2. Use a private channel:
   - GitHub Security Advisory: <https://github.com/Hesper-Labs/leakshield/security/advisories/new>
   - Email: `security@leakshield.io` (PGP key in [docs/pgp.txt](docs/pgp.txt))

We will acknowledge within 48 hours and triage within 7 days.

The following classes of issues are highest priority — please flag them clearly:

- Pathways that leak master provider keys
- Weaknesses in the KEK / DEK envelope encryption or key rotation
- Cross-tenant data leakage (RLS bypass, cache poisoning, log mixing)
- DLP filter bypass (prompt injection that fools the inspector)
- Tampering with the audit-log hash chain

## Built-in security features

- Envelope encryption (KEK ⊃ DEK ⊃ data) with pluggable KEK providers (Vault Transit, AWS KMS,
  GCP KMS, Azure Key Vault, local file)
- Argon2id hashing for virtual keys
- PostgreSQL row-level security for tenant isolation
- Append-only audit log with daily Merkle root verification
- Secret-scrubbing log middleware (redacts `Authorization`, `x-api-key`, `sk-*`, `gw_live_*`)
- Adversarial test suite gating DLP prompt edits

## Disclosed advisories

Published advisories are tracked on the GitHub Security Advisories page and referenced in
[SECURITY-ADVISORIES.md](SECURITY-ADVISORIES.md) once available.
