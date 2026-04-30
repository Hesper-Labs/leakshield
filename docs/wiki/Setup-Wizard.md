# Setup Wizard

LeakShield's first-launch UX is a five-step wizard. The whole goal is "from blank install to a
working proxied LLM call in under five minutes."

## Routing

When a visitor opens the panel root, the server checks
`GET /admin/v1/setup/status`:

- `reachable: true` and `has_admin: false` → `/onboarding/1`
- `reachable: true` and `has_admin: true`, signed-out → `/sign-in`
- `reachable: true` and `has_admin: true`, signed-in → `/dashboard`
- `reachable: false` (gateway offline) → `/onboarding/1` in *demo mode*, with a banner explaining
  that submissions will surface inline errors but the UI is browsable.

`/sign-up` is a client-facing redirect to `/onboarding/1` — there is no "sign-up vs sign-in"
choice on a fresh install.

## Step 1 — Create the admin account

A single screen: company name (slug auto-derived), your name, email, password + confirmation.
Live validation (passwords match, ≥ 8 chars). Submitting:

1. POSTs to `/api/setup/bootstrap` (the panel proxy), which forwards to
   `POST /admin/v1/auth/bootstrap`.
2. Gateway side: refuses if any admin already exists, generates a per-tenant DEK, wraps it under
   the KEK, inserts the company + admin + first-user mirror + default DLP policy in a single
   transaction.
3. Returns a JWT.
4. Panel calls `signIn("credentials", …)` so the freshly created admin is logged in before
   `/onboarding/2` mounts.

## Step 2 — Connect a provider

Four selectable tiles for OpenAI / Anthropic / Google / Azure. Once a tile is selected, a
discriminated form appears:

| Provider | Required | Optional |
|---|---|---|
| OpenAI | API key | base URL (proxy use), org id |
| Anthropic | API key | base URL |
| Google | API key, project id | — |
| Azure | API key, endpoint, API version, deployment map (model → deployment) | — |

The **Test connection** button hits `/api/admin/providers/test` which makes a cheap upstream call
(e.g. `GET /v1/models` for OpenAI). On success the green badge lists detected models. On failure
the red banner shows the upstream error verbatim.

**Save & continue** is gated on a successful test in this session OR an explicit "skip test"
toggle (some upstream test endpoints rate-limit aggressively). Save POSTs to
`/api/admin/providers`, the gateway encrypts the key under the tenant DEK, and the panel routes
to `/onboarding/3`.

## Step 3 — First user + virtual key

Two tabs:

- **Use my account** (default) — pre-fills the form with the session user's name and email. One
  button: **Generate my virtual key**.
- **Add a teammate** — full form: name, surname, email, phone, department.

Either path POSTs to `/api/admin/users` (if needed) and then `/api/admin/users/{id}/keys`.
The response contains the plaintext virtual key, which is shown **once** in a monospace `pre`
block with:

- A red one-time-show warning,
- A Copy button (clipboard write + toast),
- A Continue button gated on the user actually clicking Copy at least once. Failure to access
  `navigator.clipboard` (e.g. non-secure context) flips the gate so the user isn't stuck.

Once dismissed, the panel never re-serves the plaintext.

## Step 4 — DLP strategy

Three big cards (and an "Off" hidden behind a confirmation):

- **Off** — no inspection. Confirmation dialog with a clear warning. Useful only for evaluation.
- **Mock** — always ALLOW. The current default; flagged as "not for production".
- **Hybrid** — Presidio + LLM. Recommended once a model is available.
- **Specialized** — Llama Guard / ShieldGemma. Optional.
- **Judge** — any LLM with an admin-editable prompt. Most flexible.

When the admin picks an LLM-backed option, a model picker appears:

- For `ollama`, the panel queries Ollama's `/api/tags` to list locally pulled models.
- If the recommended model isn't pulled, the panel surfaces the exact `ollama pull <model>`
  command rather than pulling on the admin's behalf — LeakShield never downloads models.
- For `vllm` / `openai_compat`, a free-text model name with helpful presets.

An optional **"Add company-custom DLP categories"** sub-screen offers quick-start templates:

- "We have proprietary project codenames" → keyword-list editor.
- "We have customer / vendor lists" → CSV uploader (hashed at rest, evaluated via Bloom filter).
- "We want to block discussion of pending deals / M&A" → LLM-only category with a pre-written
  description the admin can refine.
- "We want to keep source code with secrets out of prompts" → toggles the built-in
  `CODE.SECRET_IN_SOURCE` category to BLOCK.

## Step 5 — Verify

A copy-paste curl snippet for the chosen provider native endpoint, with the just-issued virtual
key already substituted in:

```bash
curl http://localhost:8080/openai/v1/chat/completions \
  -H "Authorization: Bearer gw_live_xxxxxxxxxxxxxxxxxxxx" \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}'
```

The page subscribes to `/admin/v1/stream/logs` over SSE. When the first request from the new key
arrives, the page transitions (with a confetti / success animation) to the dashboard. This
"first request lit up" moment is intentionally a high-impact reward — it is the most important
UX in the entire product for OSS adoption.

## Demo mode

When the panel detects the gateway is unreachable (`fetchSetupStatus` returns
`reachable: false`), every step renders an info banner at the top:

> **Gateway is offline — demo mode**
> The Go gateway isn't reachable, so the wizard is running against stub responses. Submitting
> will surface the error inline so you can still walk through the UI. `docker compose up
> gateway` brings the backend up.

Demo mode keeps the UI browsable (so first-time visitors can evaluate the experience without
running the entire stack) while making it clear that nothing is being persisted.
