# Roadmap

LeakShield is **pre-alpha**. The `main` branch is functional end-to-end (admin bootstrap →
provider connect → virtual key issuance → live proxy with passing tests for everything wired)
but several production-critical features are still stubs. This page lists what's in, what's
pending, and what's coming next.

The authoritative source of "what shipped and what's pending" is the
[CHANGELOG](https://github.com/Hesper-Labs/leakshield/blob/main/CHANGELOG.md). This page is the
narrative version.

## Shipped on `main`

| Area | Status |
|---|---|
| Gateway: multi-protocol native proxy (OpenAI / Anthropic / Google / Azure) | ✅ |
| Gateway: chat handler with DB-backed virtual key auth + envelope decrypt + provider dispatch | ✅ |
| Gateway: admin REST (setup-status, bootstrap, login, me, providers, users, keys) | ✅ |
| Gateway: JWT (HS256) admin sessions, argon2id passwords, KEK / DEK envelope crypto, RLS | ✅ |
| Gateway: PostgreSQL schema with monthly partitioned audit log + hash chain columns | ✅ |
| Inspector: Python gRPC service, mock + hybrid strategies, Presidio + Turkish recognizers, Bloom-filter directories, judge-prompt scaffold | ✅ |
| Inspector: 30+ adversarial test cases gating DLP prompt edits | ✅ |
| Panel: Next.js 15 with shadcn/ui, light theme, onboarding wizard steps 1–3 wired end-to-end | ✅ |
| Helm chart, Docker Compose (dev + prod variants) | ✅ |
| GitHub Actions CI matrix (Go × OS, Python × version, Node × version), CodeQL, Dependabot, release workflow with cosign + SBOM | ✅ |
| Top-level docs: architecture, setup-wizard, dlp-categories, OpenAPI 3.1, curl walkthrough, this wiki | ✅ |

## In-flight (next week or so)

- **Gateway gRPC inspector client.** The chat handler currently uses an always-ALLOW
  placeholder. The wiring shape is exactly what the real client will look like; it's a
  mechanical drop-in.
- **Audit log writer.** Schema is in, every column is wired in the Go struct, and the writer
  needs to actually be called from the chat handler. Tracking under
  `gateway/internal/audit/`.
- **Onboarding wizard step 4 (DLP picker)** with template chooser (Off / Mock / Hybrid /
  Specialized / Judge) and the model selection UX that surfaces an `ollama pull` command if the
  selected model isn't pulled yet.
- **Onboarding wizard step 5 (verify).** SSE-driven first-request detection in the panel; the
  page subscribes to the live audit log and triggers a confetti / success transition when the
  first proxied request arrives.

## Soon (this quarter)

- Specialized + Judge inspector strategies (mock + hybrid ship today; specialized is Llama
  Guard 3 with a clean prompt boundary; judge is "any LLM + admin prompt").
- Streaming response filtering — toggle exists in the policy schema, runtime is the next slice.
- KMS-backed KEK providers: Vault Transit, AWS KMS, GCP KMS, Azure Key Vault. The interface
  exists; implementations and integration tests are the work.
- Universal `/v1/chat/completions` LiteLLM-style router that picks a provider based on virtual
  key policy or `model` heuristic.
- Cold-tier audit log export to S3 / MinIO as Parquet, with the schema partitioned by tenant +
  date for cheap analytics queries.
- Hash-chain verification job that recomputes the audit-log Merkle root nightly and signs the
  manifest into object storage.

## Later

- More provider adapters: Cohere, Mistral, AWS Bedrock, Vertex AI direct (currently routed
  through Google Gemini surface).
- A separate `cli/` for power-user actions (mass key rotation, audit log export, KEK rotation)
  that doesn't need the panel.
- Plugin SDK for inspector strategies — a lighter contract than the gRPC service so contributors
  can add a new strategy without writing a Python service from scratch.
- A real terraform provider for managing tenants / providers / keys / policies as code.
- A CLI for the admin REST API so ops can script everything the panel does.

## What is *not* on the roadmap

- A managed SaaS offering. LeakShield is self-host-only, by design. We will not build a hosted
  product.
- Any kind of telemetry, anonymous usage stats, or "phone home" feature. Even opt-in.
- Multi-region active-active out of the box. Multi-region works fine if you replicate Postgres,
  but we won't add an explicit multi-master mode.

## Contributing to the roadmap

The fastest way to influence what we work on next is to file an issue with a clear use case, or
to open a PR. We're a small maintainer team and tend to prioritise things that have a real user
behind them over abstract correctness work.
