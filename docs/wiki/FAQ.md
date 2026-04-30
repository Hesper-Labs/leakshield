# FAQ

## Why "LeakShield"?

The product's job is to be a shield around your LLM traffic that catches leaks. The name was
short, said what it does, and the domain was free.

## Why a separate gateway and inspector?

Because the gateway is a hot-path proxy and the inspector is a model-bound brain. Splitting them
lets each scale on the right axis (CPU for the gateway, RAM/GPU for the inspector) and lets
operators run them on different node pools. They talk over a single gRPC contract
(`proto/inspector/v1/inspector.proto`), so adding a new strategy or backend doesn't touch the
gateway.

## Why Go for the gateway?

Goroutine-per-request is the right concurrency model for a streaming proxy. `net/http` + `chi`
gives us first-class HTTP/2, real `Flusher` semantics for SSE, and trivial deployment as a single
binary. Anything Node-shaped would have struggled with the streaming-and-cancellation invariants.

## Why Python for the inspector?

The DLP ecosystem (Microsoft Presidio, Llama Guard, vLLM, llama.cpp, Ollama clients) is Python.
The inspector is glue code over those libraries; rewriting them in Go would be both pointless
and disrespectful to the underlying work.

## Why Next.js for the panel?

Server Components let us render heavy tables (analytics, audit log) on the server and stream
them. Auth.js v5 has a clean Credentials provider that fits our "the gateway owns the user table"
constraint. shadcn/ui gave us owned, customisable primitives without a black-box import.

## Does LeakShield phone home?

No. Zero telemetry, zero analytics, zero auto-checks. The gateway will not make any outbound
request that you didn't ask for: it talks to Postgres, Redis, the inspector, and whichever LLM
provider you've configured. That's it.

## Will my prompts leave my network?

Only if you've configured a provider that is itself outside your network and a request passes
DLP. If your DLP is `mock` (the default at first install), every prompt that authenticates passes
straight through to the upstream provider; flip the strategy to `hybrid` (with a real backend) to
actually inspect prompts before they leave.

## Can I use LeakShield with a model running entirely on-prem?

Yes. Set the inspector's backend to `openai_compat` and point it at your in-cluster vLLM / TGI /
Ollama OpenAI-compatible endpoint. For the *upstream* call, use any provider that has an
on-prem option (Azure OpenAI in a private cluster, AWS Bedrock with PrivateLink, or your own
endpoint). The gateway has no preference about where the upstream lives.

## Why is the default DLP strategy "mock"?

Because we never want a fresh `docker compose up` to require pulling 4 GB of model weights. The
mock strategy lets you reach the panel, walk the wizard, issue a key, and see a real proxied
request work end-to-end inside five minutes. The setup wizard prominently nudges admins to swap
to a real strategy.

## Can the admin write a "always allow" DLP prompt to bypass the system?

Not silently. The Categories editor runs every save against an adversarial test suite (~30
known-bad prompts). A new prompt that would have allowed a previously blocked prompt is
rejected with a clear diff. A determined adversarial admin can still disable DLP entirely by
setting `strategy = off` — but that's a top-level audited switch, not a hidden bypass.

## Does LeakShield support my LLM provider?

Today: OpenAI, Anthropic, Google Gemini, Azure OpenAI. The architecture is "one file per
provider in `gateway/internal/provider/`" so adding Cohere, Mistral, Bedrock, or a self-hosted
OpenAI-compatible endpoint is small, scoped work — see
[`CONTRIBUTING.md`](https://github.com/Hesper-Labs/leakshield/blob/main/CONTRIBUTING.md).

## Can I use LeakShield in front of a public-facing app, not just internal employees?

Yes — that was actually a later design consideration. The audit log doesn't care whether the
caller is an internal employee or an end user of your product. Per-virtual-key budgets and rate
limits scale to API-as-a-product use cases. The setup wizard's language assumes "first user is
the admin", but nothing in the product is gated on that being true.

## Is the wiki up to date?

Yes — it lives in the same monorepo (`docs/wiki/`) and gets pushed to the GitHub Wiki via the
`scripts/sync-wiki.sh` helper. If you spot drift, open a PR against `docs/wiki/`; the next push
will reconcile.

## How do I report a vulnerability?

Through the GitHub Security Advisory page on the repo, or by email to the maintainers — see
[SECURITY.md](https://github.com/Hesper-Labs/leakshield/blob/main/SECURITY.md). Do not file
public issues for vulnerabilities.

## Where do I get help?

GitHub Issues for bugs / feature requests, GitHub Discussions for questions / RFCs / "is anyone
else doing X" threads. The maintainer team is small and responsive but does not promise SLA-
grade turnaround.
