# Changelog

All notable changes to LeakShield are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project adheres
to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Multi-protocol native gateway with pass-through adapters for OpenAI, Anthropic,
  Google Gemini, and Azure OpenAI.
- Admin REST API: setup status, bootstrap, login, me, providers CRUD, users CRUD,
  per-user virtual key issuance.
- JWT (HS256) admin sessions; argon2id password hashing.
- Envelope encryption: per-tenant DEK wrapped by a KEK from local file (auto-
  generated 0600 in dev) or pluggable KMS (Vault / AWS / GCP / Azure stubbed).
- Virtual key format `gw_<env>_<prefix>_<secret>` with prefix-indexed lookup and
  argon2id-hashed secret. In-memory verifier cache with short positive + negative
  TTL.
- PostgreSQL schema with row-level security on every tenant-scoped table; monthly
  partitioned audit log with hash-chain columns.
- Inspector gRPC service in Python with mock + hybrid strategies. Hybrid combines
  Microsoft Presidio (with Turkish-aware recognizers — TC kimlik checksum, IBAN
  MOD-97, GSM, address heuristic) with company-custom categories (keyword lists,
  regex, document fingerprints, hashed customer / employee directory via Bloom
  filter, LLM-only category descriptions).
- Adversarial test suite gating DLP prompt edits (~30 known-bad prompts).
- Next.js 15 admin panel: light-theme defaults, shadcn/ui primitives owned in
  the repo, TanStack Query, Zustand, Recharts, Monaco for the policy editor,
  next-intl with English + Turkish messages, Auth.js v5 against the Go gateway.
- Onboarding wizard steps 1–3 wired end-to-end: create admin, connect provider
  (with test connection), first user + one-time virtual key reveal. Steps 4–5
  remain placeholders.
- Helm chart (`deploy/helm/leakshield`) covering gateway, inspector, panel,
  ingress, optional bundled Postgres / Redis, network policy, HPA, secrets.
- GitHub Actions: CI matrix (Go, Python, Node), CodeQL, Dependabot, release
  workflow with cosign-signed images and SPDX SBOM.
- OpenAPI 3.1 spec (`docs/openapi.yaml`) and curl walkthrough (`docs/api.md`).
- Top-level docs: architecture, setup-wizard, dlp-categories.

### Pending
- gRPC inspector client on the Go side (chat handler currently uses an
  always-ALLOW placeholder; the wiring is shaped for a mechanical drop-in).
- Audit log writer (schema is in; the writer comes next).
- Onboarding wizard steps 4 (DLP picker) and 5 (verify with SSE-driven first-
  request detection).
- Specialized + Judge inspector strategies (mock + hybrid ship today).
- Streaming response filtering (toggle present, runtime not yet implemented).
- Cold tier Parquet export to S3 / MinIO.
- Hash-chain audit verification job.
- KMS-backed KEK providers (Vault Transit, AWS / GCP / Azure KMS).
- Universal `/v1/chat/completions` LiteLLM-style router.

[Unreleased]: https://github.com/Hesper-Labs/leakshield/commits/main
