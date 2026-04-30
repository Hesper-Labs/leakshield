# Deployment: Docker Compose

The default deploy mode for development and small single-node production setups. The repo ships
two compose files:

- [`docker-compose.yml`](https://github.com/Hesper-Labs/leakshield/blob/main/docker-compose.yml) — dev defaults: bundled Postgres, Redis, gateway,
  inspector (mock backend), panel. Optional `local-llm` profile adds Ollama.
- [`deploy/compose/docker-compose.prod.yml`](https://github.com/Hesper-Labs/leakshield/blob/main/deploy/compose/docker-compose.prod.yml) — production-flavoured: external Postgres / Redis,
  TLS termination via traefik or caddy (commented sample), env-file-driven KEK, restart
  policies, and log-driver tuning.

## Dev: `docker compose up`

```bash
docker compose up -d
```

Brings up:

- `postgres:16-alpine` — bundled, named volume.
- `redis:7-alpine` — bundled, named volume.
- `inspector` — Python service with `LEAKSHIELD_INSPECTOR_BACKEND=mock`.
- `gateway` — Go binary; runs `migrate up` automatically on first start (planned: today the
  user runs it manually with `docker compose exec gateway leakshield migrate up`).
- `panel` — Next.js production build serving on `:3000`.

The default `LEAKSHIELD_KEK_FILE=/var/lib/leakshield/kek` lives on the named volume so the KEK
survives container restarts. **Back this up.** Losing the KEK loses every encrypted master
provider key.

## Opt-in local LLM

```bash
docker compose --profile local-llm up -d
```

Adds an Ollama container with a persistent volume. **Models are not pulled automatically**;
shell into the container and pull what you need:

```bash
docker compose exec ollama ollama pull qwen2.5:3b-instruct
docker compose exec ollama ollama pull llama-guard3:1b
```

Then in the panel: Settings → DLP → Backend → Ollama, and pick a model.

## Production single-node

`deploy/compose/docker-compose.prod.yml` expects external Postgres + Redis and reads secrets
from a separate env file you keep out of git:

```bash
# ~/.config/leakshield/.env  (mode 0600, not in git)
LEAKSHIELD_DATABASE_URL=postgres://leakshield:<password>@db.internal:5432/leakshield?sslmode=require
LEAKSHIELD_REDIS_URL=rediss://cache.internal:6380/0
LEAKSHIELD_KMS_PROVIDER=local
LEAKSHIELD_KEK_FILE=/etc/leakshield/kek
LEAKSHIELD_JWT_SECRET=<64 random bytes>
LEAKSHIELD_PROD=1
```

Then:

```bash
docker compose -f deploy/compose/docker-compose.prod.yml --env-file ~/.config/leakshield/.env up -d
```

## Operational essentials

- **Migrations.** Idempotent. Run `leakshield migrate up` after any image upgrade. The gateway
  will refuse to start if the schema version doesn't match.
- **KEK.** 32-byte file, 0600. Back it up. Losing it loses every master provider key.
- **JWT secret.** 64-byte file, 0600. Rotating this invalidates all admin sessions but does not
  affect data.
- **Backups.** `pg_dump` of the Postgres database, plus the KEK file. Both are required to
  restore.
- **Upgrades.** `docker compose pull && docker compose up -d`. The gateway exits cleanly on
  SIGTERM after draining in-flight requests; the panel is stateless.

## Networking

By default the dev compose binds:

- `:3000` panel
- `:8080` gateway public proxy + admin REST
- `:8090` admin-only REST (separate so production can lock it down)
- `:5432` Postgres
- `:6379` Redis
- `:50051` inspector gRPC (loopback only on prod compose)
- `:11434` Ollama (loopback only on prod compose)

In production you typically front `:3000` and `:8080` behind a reverse proxy (traefik or caddy
sample blocks live in the prod compose file) with TLS, and never expose the others.

## Verifying it works

```bash
curl http://localhost:8080/healthz                     # → { "status": "ok" }
curl http://localhost:8080/admin/v1/setup/status       # → { has_admin: false, ... }
curl http://localhost:3000/                            # → 307 → /onboarding/1
```

If `setup/status` returns `reachable: false` from the panel route
(`/api/setup/status`) but the gateway responds directly, that's the
`GATEWAY_INTERNAL_URL` env var — see
[Self-Hosting Guide](Self-Hosting-Guide#panel--gateway-networking).
