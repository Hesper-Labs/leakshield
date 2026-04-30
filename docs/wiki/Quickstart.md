# Quickstart

This page gets you from zero to a successful proxied LLM call in under five minutes. If anything
on this page does not work, please file an issue — that's the bar.

## Prerequisites

- Docker (Compose v2) or a recent Go + Python + Node toolchain.
- Optional but recommended: an OpenAI / Anthropic / Google / Azure API key to actually proxy
  requests through. Without one you can still walk through the panel and DLP editor; you'll just
  see test-connection failures on the provider step.

## 1. Get the source

```bash
git clone https://github.com/Hesper-Labs/leakshield
cd leakshield
```

## 2. Start the stack

```bash
docker compose up -d
```

That brings up Postgres, Redis, the gateway (Go), the inspector (Python — `mock` strategy by
default), and the panel (Next.js). It does **not** pull any LLM weights. The default DLP strategy
allows everything until you swap it.

To get a real local LLM behind the inspector, opt in:

```bash
docker compose --profile local-llm up -d
```

That adds an Ollama service. LeakShield does not download models; pull whichever model you want:

```bash
ollama pull qwen2.5:3b-instruct      # for the LLM judge
ollama pull llama-guard3:1b          # for the specialized strategy
```

## 3. Open the panel

Visit <http://localhost:3000>. On a fresh install you land directly on the create-admin form.
Fill in:

- Company name (the slug auto-derives, e.g. "Acme" → `acme`)
- Your name
- Email
- Password (≥ 8 characters)

Submit. You're now signed in as a super-admin and the wizard advances to the provider step.

## 4. Connect a provider

Pick OpenAI / Anthropic / Google / Azure on the wizard's tile selector, paste the API key, and
click **Test connection**. If the upstream call succeeds, the green badge shows the detected
models. Save and continue.

Internally the gateway encrypts the key under your tenant DEK before storing it; the plaintext
never lands in the database.

## 5. Issue your first virtual key

The wizard pre-fills "Use my account" with the email + name from step 3. Click **Generate my
virtual key**. The plaintext is shown **once** — copy it now, the panel will not show it again.

## 6. Make a real request

Replace `<KEY>` below with the plaintext virtual key:

```bash
# OpenAI native
curl http://localhost:8080/openai/v1/chat/completions \
  -H "Authorization: Bearer <KEY>" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"Hello"}]}'
```

```bash
# Anthropic native (works with Claude Code CLI directly):
export ANTHROPIC_BASE_URL=http://localhost:8080/anthropic
export ANTHROPIC_API_KEY=<KEY>
claude
```

```bash
# Google Gemini native
curl "http://localhost:8080/google/v1beta/models/gemini-2.0-flash:generateContent?key=<KEY>" \
  -H "Content-Type: application/json" \
  -d '{"contents":[{"parts":[{"text":"Hello"}]}]}'
```

The panel's `Logs` page shows the request landing in real time over SSE.

## What's next

- Tune the DLP strategy: [DLP Categories](DLP-Categories), [Policy Editor](Policy-Editor).
- Swap the inspector backend from `mock` to `ollama` to get real DLP: see the inspector config
  in [Architecture](Architecture#inspector).
- Move from local 0600 KEK to a real KMS: [Encryption and KEK](Encryption-and-KEK).
- Deploy beyond a laptop: [Deployment: Kubernetes](Deployment-Kubernetes).
