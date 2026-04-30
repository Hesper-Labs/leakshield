# Self-Hosting Guide

This is the operator's-eye-view of running LeakShield for your team. If you're just trying it on
a laptop, [Quickstart](Quickstart) is the right starting point. This page covers what changes
when you graduate from "demo on my MacBook" to "the people I work with depend on this".

## What you actually have to run

Three services and two backing stores:

| What | Why | Sizing rule of thumb |
|---|---|---|
| Gateway (Go) | Hot path. Auth, routing, encryption, audit. | CPU-bound. ~1 vCPU per ~1k RPS. Stateless: scale horizontally. |
| Inspector (Python) | DLP brain. | Memory-bound + (optionally) GPU-bound. One instance per GPU is fine until throughput hurts. |
| Panel (Next.js) | Admin UI. | Trivial. 1 small pod. |
| PostgreSQL ≥ 15 | Source of truth. | A modest managed instance handles tens of thousands of users; tune `shared_buffers` once you cross 10k RPS. |
| Redis 7 | Rate limit + verdict cache. | One small replica for ≤ 5k RPS. |

Optional, depending on your DLP strategy choice:

- An **Ollama / vLLM / llama.cpp** server with whichever model the panel's DLP picker is set to.
- A **MinIO / S3** bucket for the cold-tier audit log (planned).

## Network topology

```
internet  ──TLS──►  ingress (nginx / traefik / caddy / ALB)
                           │
                           ├──► panel    (3000)        ◄── admins
                           │
                           ├──► gateway  (8080)        ◄── employee SDKs
                           │       │
                           │       ├──► inspector (50051, internal-only)
                           │       │
                           │       ├──► postgres (5432, internal-only)
                           │       │
                           │       └──► redis    (6379, internal-only)
                           │
                           └──► (optional) admin-only on (8090) behind
                                a separate VPN / SSO gate
```

The gateway also makes outbound calls to whichever LLM provider domains you've configured (api.openai.com, api.anthropic.com, generativelanguage.googleapis.com, *.openai.azure.com).
NetworkPolicy / egress firewall those to exactly that set.

## Panel ↔ gateway networking

The panel does some fetches server-side (Auth.js Credentials provider, the SSE log proxy, the
admin proxy routes). The env var `GATEWAY_INTERNAL_URL` controls where those go. In Docker
compose it's `http://gateway:8080` (the in-network DNS name); in Kubernetes it's the gateway
Service name; on a laptop running the panel locally against a non-Docker gateway it's
`http://127.0.0.1:8080`.

The browser-side fetches use `NEXT_PUBLIC_API_URL`, which must be the externally reachable
gateway URL (e.g., `https://leakshield.example.com`).

## Secrets you need to keep

| Secret | What it protects | What happens if you lose it |
|---|---|---|
| KEK | All DEKs, transitively all master provider keys. | Every encrypted blob is unreadable. **Back this up.** |
| JWT secret | Admin sessions. | All admins are logged out; data unaffected. Rotate freely. |
| Database password | Everything. | Standard DB-loss story; restore from `pg_dump`. |
| Provider master keys | Calls upstream LLMs on your dime. | Rotate at the provider, then through the panel. |

## Backups

`pg_dump` of the database **plus** the KEK is the minimum. Without the KEK, the dump is useless;
without the dump, the KEK is just a random number.

```bash
# nightly
pg_dump --dbname=$LEAKSHIELD_DATABASE_URL --format=c --file=/backup/leakshield-$(date +%F).dump
cp ~/.leakshield/kek /backup/kek-$(date +%F)
```

In Kubernetes, use a tool that can snapshot the StatefulSet PVC and a separate sealed
SecretBackup for the KEK.

## Upgrades

LeakShield uses semver. **Patch** versions ship fixes only; **minor** versions can add new
endpoints / panels / strategies; **major** versions can have breaking schema changes that need a
migration.

```bash
docker compose pull && docker compose up -d                   # Compose
helm upgrade leakshield deploy/helm/leakshield/ -f values.yaml  # Helm
```

The gateway runs migrations on startup if `--auto-migrate=true` (default off in `--prod`); in
production, run them as a one-shot job before swapping pods.

## Observability stack

Recommended minimum:

- **Logs**: forward stdout JSON from all three services into your aggregator. Filter on
  `request_id` to correlate.
- **Metrics**: scrape `:9090` from each service. The chart exposes a `ServiceMonitor` when
  Prometheus Operator is around. Dashboards: a Grafana folder is on the roadmap.
- **Tracing**: point `LEAKSHIELD_OTLP_ENDPOINT` at your collector. Traces span panel → gateway
  → inspector → upstream provider so a slow request is debuggable end-to-end.

## Capacity planning

- A bursty 100-user team: a single 1-vCPU gateway pod, a 4 GB Postgres, a 1 GB Redis. The
  inspector depends on your model: Llama Guard 1B on CPU is fine; Qwen 2.5 3B as a judge wants a
  4-core / 16 GB host or a small GPU.
- A 1000-user team: 3 gateway pods behind an HPA, a 16 GB Postgres with read replicas only if
  analytics get heavy, a 4 GB Redis. Inspector probably wants a real GPU at this point.
- Beyond that: shard tenants across deploys. A single LeakShield install is multi-tenant but
  scaling Postgres past one box is more work than running two installs.

## What gets logged about prompts

By default: only the SHA-256 hash and a redacted preview. The full prompt is **not** stored.

If a tenant flips `policy.audit_full_prompt = true`, the full prompt is stored encrypted under
the tenant DEK in `audit_logs.prompt_encrypted`. KVKK / GDPR right-to-erasure becomes a single
`DELETE` over a hash + categories index — see [Encryption and KEK](Encryption-and-KEK).

## Common runbook entries

- **"My virtual key is rejected."** Check `virtual_keys` for `revoked_at`; check
  `expires_at`; check the prefix matches what you handed out.
- **"DLP is blocking everything legitimate."** The Categories editor's test harness shows which
  category fired. Frequently it's a regex with too-loose bounds. Adjust, redeploy.
- **"Provider is returning 401 / 403."** Run `Test connection` from the panel against the stored
  master key. If it fails, the upstream key has been rotated or revoked.
- **"Postgres is full."** The audit log is the largest table by far. Drop a partition older
  than your retention SLA: `ALTER TABLE audit_logs DETACH PARTITION audit_logs_2026_03; DROP
  TABLE audit_logs_2026_03;`. Cold-tier export is on the roadmap.
