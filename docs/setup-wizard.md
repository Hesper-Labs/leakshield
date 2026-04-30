# Setup Wizard

The first time an admin opens the panel after `docker compose up`, the gateway routes them to a
five-step setup wizard. Goal: from blank install to a working proxied LLM call in under five
minutes.

## Steps

### 1. Bootstrap the root admin

The gateway detects that no admin exists and forces sign-up. The form takes:

- Company name + slug
- Admin email + password
- Optional: enable invite-only mode

`POST /admin/v1/auth/bootstrap` creates the company, the super admin, the per-tenant DEK
(wrapped with the configured KEK), and a default `mock` DLP policy.

### 2. Connect a provider

Four large tiles for OpenAI / Anthropic / Google / Azure. Click one to expand the form:

- **OpenAI**: `api_key` (paste, masked), optional `base_url` (proxy use), optional `org_id`
- **Anthropic**: `api_key`, optional `base_url`
- **Google**: `api_key`, `project_id`
- **Azure**: `endpoint`, `api_key`, `api_version`, deployment map

The "Test connection" button performs a cheap call (e.g., `GET /v1/models` for OpenAI) and either
shows a green check with the detected model list, or surfaces the upstream error (auth /
network / quota) inline.

The admin then picks which models to allow. Defaults to the cheapest tier of each family. Per-1k
token prices appear next to each model — pulled from `provider_models`.

### 3. Create the first user

Two paths:

- **"Use my own account for testing"** — creates a `User` from the admin's email and immediately
  generates a virtual key. The key is shown once in a copy-to-clipboard panel; subsequent reads
  return only the prefix.
- **"Add a teammate"** — name / surname / email / phone / department form, same one-time key
  flow at the end.

### 4. Choose a DLP strategy

Three big cards, picked in this order of explanatory power:

1. **Off** — no inspection. Useful only for evaluation; warned about prominently.
2. **Mock** — always ALLOW. The current default; flagged as "not for production".
3. **Hybrid (Presidio + LLM)** — recommended once a model is available.
4. **Specialized DLP classifier** — Llama Guard 3, ShieldGemma, etc.
5. **LLM Judge with custom prompt** — most flexible.

When the admin picks one of the LLM-backed options, a model picker appears:

- Show the configured backend (`mock`, `ollama`, `vllm`, ...).
- For `ollama`: query `GET /api/tags` to list models the user has already pulled.
- If none of the recommended models are pulled, surface the exact `ollama pull ...` command
  rather than pulling on the admin's behalf — LeakShield never downloads models.
- For `vllm` / `openai_compat`: text input for the model name with helpful presets.

For the Judge strategy, the admin lands on the policy editor (see `policy-editor.md`).

After picking a strategy, an optional **"Add company-custom categories"** screen offers
quick-start templates the admin can accept or skip:

- "We have proprietary project codenames" → keyword-list editor.
- "We have customer / vendor lists" → CSV uploader (hashed at rest, evaluated via Bloom filter).
- "We want to block discussion of pending deals / M&A" → LLM-only category with a
  pre-written description the admin can refine.
- "We want to keep source code with secrets out of prompts" → toggles the built-in
  `CODE.SECRET_IN_SOURCE` category to BLOCK.

Categories can also be added later from Policy → Categories. See
[dlp-categories.md](dlp-categories.md) for the full mechanism reference.

### 5. Verify

A copy-paste curl snippet for the chosen provider native endpoint (preferring the one the admin
just connected):

```bash
curl http://localhost:8080/openai/v1/chat/completions \
  -H "Authorization: Bearer gw_live_xxxxxxxxxxxxxxxxxxxx" \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}'
```

The page subscribes to `/admin/v1/stream/logs` over SSE. When the first request from the new
key arrives, the page transitions (with a confetti / success animation) to the dashboard. This
"first request lit up" moment is intentionally a high-impact reward — it is the most important
UX in the entire product for OSS retention.
