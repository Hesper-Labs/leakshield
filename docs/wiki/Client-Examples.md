# Client Examples

LeakShield doesn't ship a client SDK. The whole point of the multi-protocol native gateway is
that **every existing SDK and CLI works unchanged** as long as you change the base URL and the
API key. This page collects the one-line tweak per popular client.

In every example below, replace `https://leakshield.example.com` with your gateway's URL and
`<KEY>` with a virtual key issued from the panel.

## Claude Code CLI

```bash
export ANTHROPIC_BASE_URL=https://leakshield.example.com/anthropic
export ANTHROPIC_API_KEY=<KEY>
claude
```

Native Anthropic protocol — `cache_control`, `tool_use`, the `system` array, vision blocks all
pass through unchanged.

## OpenAI Python SDK

```python
from openai import OpenAI
client = OpenAI(
    base_url="https://leakshield.example.com/openai/v1",
    api_key="<KEY>",
)
client.chat.completions.create(model="gpt-4o-mini", messages=[...])
```

## Anthropic Python SDK

```python
from anthropic import Anthropic
client = Anthropic(
    base_url="https://leakshield.example.com/anthropic",
    api_key="<KEY>",
)
client.messages.create(model="claude-sonnet-4-6", messages=[...])
```

## Google Gemini Python SDK

```python
import google.generativeai as genai
genai.configure(
    api_key="<KEY>",
    transport="rest",
    client_options={"api_endpoint": "https://leakshield.example.com/google"},
)
model = genai.GenerativeModel("gemini-2.0-flash")
model.generate_content("Hello")
```

## Azure OpenAI

```python
from openai import AzureOpenAI
client = AzureOpenAI(
    azure_endpoint="https://leakshield.example.com/azure",
    api_key="<KEY>",
    api_version="2024-08-01-preview",
)
client.chat.completions.create(model="<deployment-or--placeholder>", messages=[...])
```

The gateway accepts either a real deployment name or `-` as a placeholder; in the placeholder
form it looks up `body.model` in the company's Azure deployment map.

## Cursor / Continue.dev / Aider

Cursor: Settings → Models → Edit → Custom OpenAI Base URL → `https://leakshield.example.com/openai/v1`.

Continue.dev: in `~/.continue/config.json`, set the model's `apiBase` to
`https://leakshield.example.com/openai/v1` and `apiKey` to your virtual key.

Aider: `aider --openai-api-base https://leakshield.example.com/openai/v1 --openai-api-key <KEY> ...`.

## Vercel AI SDK

```ts
import { createOpenAI } from "@ai-sdk/openai";
const openai = createOpenAI({
  baseURL: "https://leakshield.example.com/openai/v1",
  apiKey: process.env.LEAKSHIELD_KEY!,
});
```

## LangChain

```python
from langchain_openai import ChatOpenAI
llm = ChatOpenAI(
    base_url="https://leakshield.example.com/openai/v1",
    api_key="<KEY>",
    model="gpt-4o-mini",
)
```

## curl (for debugging)

```bash
# OpenAI shape
curl https://leakshield.example.com/openai/v1/chat/completions \
  -H "Authorization: Bearer <KEY>" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}'

# Anthropic shape
curl https://leakshield.example.com/anthropic/v1/messages \
  -H "x-api-key: <KEY>" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{"model":"claude-haiku-4-5-20251001","max_tokens":50,"messages":[{"role":"user","content":"hi"}]}'

# Google shape (key as query param)
curl "https://leakshield.example.com/google/v1beta/models/gemini-2.0-flash:generateContent?key=<KEY>" \
  -H "Content-Type: application/json" \
  -d '{"contents":[{"parts":[{"text":"hi"}]}]}'
```

## Streaming

Streaming works on every endpoint that supports it natively. The gateway forwards SSE bytes
unchanged on the response side; only the request-side prompt has been inspected. Same flag, same
shape: `stream: true` on OpenAI / Anthropic / Azure, `:streamGenerateContent` on Google.

## What you do *not* need to change

- Code that handles tool / function calls.
- Code that handles `cache_control` (Anthropic prompt caching).
- Code that handles `logprobs` (OpenAI).
- Code that handles `safetySettings` / grounding (Google).
- Streaming consumers.
- Token counters.

If something doesn't work after just changing the base URL and key, that's a bug in the gateway's
adapter, not something you should work around. Open an issue with a minimal reproducer.
