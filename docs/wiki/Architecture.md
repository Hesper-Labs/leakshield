# Architecture

This page goes one level below the README diagram and explains how the gateway, inspector, panel,
and storage interact. The intended audience is anyone who needs to operate, secure, or extend
LeakShield in production.

## Components

| Component | Tech | Responsibility |
|---|---|---|
| `gateway` | Go (chi, pgx, Redis, gRPC) | Public proxy + admin REST API + worker. Multi-protocol native endpoints. |
| `inspector` | Python (asyncio, gRPC, Presidio, optional vLLM / Ollama) | DLP decisions per request. Pluggable strategies and backends. |
| `panel` | Next.js 15 + TypeScript + Tailwind v4 + shadcn/ui + Recharts + Monaco | Admin web app. |
| `proto/` | protobuf | Inspector contract; the only cross-language interface. |
| Postgres | tenant data, audit log, usage rollups | RLS on every tenant-scoped table; monthly partitions on the audit log. |
| Redis | rate limit + verdict cache + revocation pub-sub | optional but recommended in production. |

## Data plane

```
client SDK                     Gateway (Go)                   Inspector (Python)
   │                                │                                 │
   ├── HTTP request ───────────────►│                                 │
   │                                ├── auth (virtual key) ───────────│
   │                                ├── rate limit (Redis Lua) ───────│
   │                                ├── load policy (Postgres) ───────│
   │                                ├── gRPC InspectPrompt ──────────►│
   │                                │                              ┌──┤ strategy
   │                                │                              │  │ → mock / hybrid /
   │                                │                              │  │   specialized / judge
   │                                │                              └─►│ backend
   │                                │                                 │ → mock / ollama / vllm /
   │                                │                                 │   llamacpp / openai_compat
   │                                ├──────── verdict ◄───────────────┤
   │                                │                                 │
   │                                ├── if ALLOW: forward to provider │
   │                                │   (HTTP/2 multiplexed, SSE)     │
   │                                ├── stream response back ─────────│
   │                                ├── audit_log INSERT              │
   ◄────── SSE response ────────────┤                                 │
```

## Multi-protocol native gateway

Each provider is exposed on its native API URL prefix. Existing SDKs and CLIs work with only a
`base_url` change — no wrapper, no translation layer that might lose features.

| Provider | Endpoint prefix | Pass-through capabilities |
|---|---|---|
| OpenAI | `/openai/v1/*` | logprobs, function calling, vision, structured outputs, tools |
| Anthropic | `/anthropic/v1/*` | `cache_control`, `tool_use`, `system` array, vision, batches |
| Google Gemini | `/google/v1beta/*` | grounding, safetySettings, codeExecution, function declarations |
| Azure OpenAI | `/azure/openai/deployments/*` | deployment routing, content-filter metadata |
| Universal router | `/v1/chat/completions` (OpenAI shape, optional) | LiteLLM-style; routes by virtual key policy or `model` heuristic |

The gateway is **pass-through**. It parses the request body to extract messages for inspection,
swaps the auth header, and (for `MASK`) rewrites only the message content. Everything else is
forwarded byte-for-byte. New provider features keep working without adapter changes; only the
audit metadata may lag until support lands.

## Inspector

The inspector is a separate gRPC service so the gateway hot path is in Go (low latency, easy
concurrency) while the LLM-heavy work runs in Python where the ecosystem is.

Strategies (chosen per tenant from the panel; the model is the admin's choice too):

- **`mock`** — always ALLOW. Default at first install so the rest of the stack is exercisable
  without an LLM. The setup wizard nudges admins to switch.
- **`hybrid`** — Presidio (regex / NER, including Turkish recognizers) for fast classification,
  escalating ambiguous cases to a user-chosen LLM. Built-in recognizers cover universal PII +
  credentials + source-code-embedded secrets. Per-tenant categories layer on top.
- **`specialized`** — purpose-built DLP classifiers (Llama Guard 3, ShieldGemma…). Optional.
- **`judge`** — any general LLM driven by an admin-editable prompt. The prompt has an immutable
  scaffold and an adversarial test suite gating deploys.

Backends (independent of strategy):

| Backend | Status | Notes |
|---|---|---|
| `mock` | shipped | always ALLOW, no external calls |
| `ollama` | shipped | talks to a local Ollama HTTP API; works with any model you've pulled |
| `vllm` | stub | OpenAI-compatible HTTP server, GPU production |
| `llamacpp` | stub | embedded `llama-cpp-python`, in-process |
| `openai_compat` | stub | LM Studio, text-generation-webui, LiteLLM |

LeakShield never downloads models for you. The inspector's `mock` backend lets the gateway work
end-to-end without any model; switch to `ollama` once you pull whatever model you want.

## Built-in vs. company-custom DLP categories

DLP is rarely "just PII." Every company has its own definition of "do not let this leave the
building." LeakShield's strategies layer two category sets:

1. **Built-in** — PII (name, email, phone, TC kimlik, IBAN, passport, address, DOB, credit card,
   amounts), credentials (OpenAI, Anthropic, AWS keys, PEM blocks, generic high-entropy tokens),
   and source-code-embedded secrets.
2. **Company-custom**, declared by the admin, with any combination of:
   - keyword lists (project codenames, internal terminology),
   - regex patterns (internal ticket / account ID formats),
   - document fingerprints (`"Confidential — Internal Only"`),
   - LLM-only categories described in plain English ("any pending M&A discussion"),
   - hashed customer / employee directories evaluated via Bloom filters so the raw list never
     enters LLM context.

Each category carries a severity (`ALLOW` / `MASK` / `BLOCK`) and is encrypted at rest under the
tenant DEK alongside provider keys, since the rules themselves describe what the company
considers secret.

Reference: [DLP Categories](DLP-Categories).

## Security posture

- **Envelope encryption**: KEK ⊃ DEK ⊃ data. KEK pluggable: Vault Transit, AWS / GCP / Azure KMS,
  or local 0600 file (dev). Per-tenant DEK encrypts master provider keys and (optionally) the
  full audit-log prompts. See [Encryption and KEK](Encryption-and-KEK).
- **Virtual keys**: `gw_<env>_<8-char-prefix>_<32-char-secret>`. Prefix indexed for O(1) lookup,
  secret hashed with argon2id (m=64MiB, t=3, p=2). See [Virtual Keys](Virtual-Keys).
- **JWT admin sessions**: HS256 with a 64-byte secret in the `~/.leakshield/jwt.secret` file
  (auto-generated on first run, refused under `--prod` unless explicit env var is set).
- **Tenant isolation**: PostgreSQL row-level security plus application-level filtering. Each
  request opens a transaction with `SET LOCAL app.tenant_id`.
- **Audit log**: append-only, monthly partitioned, hash-chained for tamper evidence.
- **Secret scrubbing**: log middleware redacts `Authorization`, `x-api-key`, `sk-*`, `gw_live_*`
  patterns; CI tests planted secrets to verify redaction.

## Failure modes

| Scenario | Behavior |
|---|---|
| Inspector down | per-policy `fail_mode` (default `closed` → 503). Circuit breaker shields the down service. |
| Provider 5xx | Single retry with jitter on idempotent failures, then 502 to client. No automatic provider failover (would silently change billing / data residency). |
| Provider 429 | Respect `Retry-After`; per-(company, provider) circuit breaker. |
| Postgres down | Verified-key LRU keeps existing keys working briefly; audit writes go to a bounded ring buffer; if the buffer fills, requests fail closed (audit loss is unacceptable). |
| Redis down | Rate limit fails open with a warning + alert. Auth path does not depend on Redis. |
| Inspector OOM / model crash | Python supervisor restarts the worker; queued requests time out and apply `fail_mode`. |

## Observability

- **Tracing**: OpenTelemetry across Go and Python. Trace context propagated via gRPC metadata
  and outbound HTTP headers.
- **Metrics**: Prometheus — request counts, latency histograms, blocked counts, in-flight
  gauges, GPU queue depth.
- **Logging**: `slog` (Go) and `structlog` (Python) emitting JSON; never log prompts, only
  hashes and decisions.

## Deployment topology

- **Dev / single-node**: `docker compose up`. Optional `docker compose --profile local-llm up`
  brings up Ollama too.
- **Production**: Helm chart in `deploy/helm/leakshield/`. Gateway scales horizontally on CPU /
  in-flight requests; inspector scales vertically first (bigger GPU) then horizontally.
- **Secrets**: KMS / Vault required in `--prod` mode; the gateway refuses to start with an
  env-var KEK in production.

See [Deployment: Docker Compose](Deployment-Docker-Compose) and
[Deployment: Kubernetes](Deployment-Kubernetes).
