# Comparison

LeakShield sits at the intersection of "AI gateway" and "DLP for LLMs". Most adjacent tools do
one half well. This page is a fair, factual look at what overlaps and what doesn't.

## Quick-glance matrix

| Capability | **LeakShield** | LiteLLM | Lakera | Llama Guard alone | Portkey |
|---|---|---|---|---|---|
| Self-hosted, no SaaS dependency | yes | yes | no (cloud) | yes (just a model) | partial (SaaS-first) |
| Provider-native multi-protocol proxy | yes (OpenAI / Anthropic / Google / Azure) | OpenAI shape only | n/a | n/a | yes (OpenAI shape) |
| Encrypted master provider keys (KEK ⊃ DEK, KMS-pluggable) | yes | partial | n/a | n/a | partial |
| Per-employee virtual keys with budgets and audit | yes | partial | no | no | yes |
| Pluggable local LLM as DLP judge | yes (any OpenAI-compatible / Ollama / vLLM / llama.cpp) | n/a | n/a | one classifier model | n/a |
| Editable DLP prompt + adversarial test gate | yes | n/a | n/a | n/a | n/a |
| Hybrid Presidio + LLM with TC kimlik / IBAN / GSM recognizers | yes | n/a | partial | no | n/a |
| Tamper-evident audit hash chain | yes | no | partial | no | partial |
| Multi-tenant (PostgreSQL row-level security) | yes | no | n/a | n/a | yes |

## When LiteLLM is the right tool

LiteLLM is excellent at "I want to swap providers behind one OpenAI-shaped endpoint". If your
need is "let our app code stay OpenAI-shaped while the team experiments with Claude / Gemini /
Llama", LiteLLM is older, more battle-tested, and ships a much larger provider catalog. It does
not, however, treat DLP as a first-class concern, and its key-management story is light.

A reasonable stack: LiteLLM in front of LeakShield, where LiteLLM does provider routing for the
common OpenAI shape and LeakShield does DLP + virtual keys + audit. This works because
LeakShield's `/v1/chat/completions` is OpenAI-compatible.

## When Lakera is the right tool

If you want a managed SaaS that solves prompt injection and PII at the edge with no infra of
your own, and your prompts are allowed to leave your network, Lakera Guard is the path of least
resistance. The trade-off is exactly what most LeakShield users come for: prompts leaving the
network is the thing they want to prevent.

## When Llama Guard alone is enough

If your only need is "don't let the LLM say something egregious" and you have no need for
multi-protocol routing, key custody, virtual keys, audit, or company-custom categories, you can
run Llama Guard 3 directly in your inference path. It's a great model for what it does. It's
also one strategy of four inside LeakShield's `inspector/` — and the only "specialized"
strategy LeakShield ships.

## When Portkey is the right tool

If you've already paid for Portkey or want their hosted dashboards, it covers a lot of the same
gateway / virtual-key / observability ground. The DLP story is smaller (no admin-editable
LLM-as-judge, no Turkish-aware recognizers, no Bloom-filter customer-name matching), but the
operational maturity is higher.

## Where LeakShield is genuinely differentiated

The combination, not any single feature, is the differentiator:

1. **Multi-protocol native** so existing client code (Claude Code CLI, Anthropic SDK, Gemini SDK)
   keeps using its native protocol instead of being squeezed through OpenAI shape.
2. **DLP that the admin can shape** — keyword lists, regex, document fingerprints, hashed
   directories, LLM-only category descriptions, all defined per tenant, with an adversarial test
   suite preventing "always allow" prompts from shipping.
3. **End-to-end self-hosted** — your prompts, the master provider keys, the encrypted DEKs, the
   audit log, and the panel all live inside your network. There is no managed offering and no
   telemetry phone-home.
4. **PostgreSQL row-level security** doing tenant isolation at the database layer, not just the
   application layer.

If your need is exactly one of those four, you can usually pick a more mature single-purpose
tool. If your need is "I want all four in one self-hosted package", LeakShield is built for
that gap.

## Honest weaknesses (today)

- **Pre-alpha.** Several roadmap items are stubs (`# planned` in
  [API Reference](API-Reference)), the gateway → inspector gRPC client is currently a placeholder
  always-ALLOW, and the audit-log writer is wired but not yet emitting all the metadata the
  schema captures.
- **Smaller provider catalog than LiteLLM.** OpenAI / Anthropic / Google / Azure are the four
  natively supported providers. Cohere, Mistral, Bedrock are on the roadmap.
- **Helm chart not yet battle-tested at scale.** Production deployments to date are docker-compose
  on a single VM. The chart is real but the war stories aren't there yet.
- **No managed offering.** That's a feature for our intended audience and a non-starter for
  others. Pick accordingly.
