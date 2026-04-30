<div align="center">

<img src="https://raw.githubusercontent.com/Hesper-Labs/leakshield/main/assets/logo.png" alt="LeakShield" width="200" />

# LeakShield

**API Gateway. Secret Guard. Built for Safety.**

[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Status](https://img.shields.io/badge/status-pre--alpha-orange.svg)](#status--roadmap)
[![Go](https://img.shields.io/badge/Go-1.23-00ADD8.svg?logo=go&logoColor=white)](gateway/go.mod)
[![Python](https://img.shields.io/badge/Python-3.11%2B-3776AB.svg?logo=python&logoColor=white)](inspector/pyproject.toml)
[![Node](https://img.shields.io/badge/Node-20%2B-339933.svg?logo=node.js&logoColor=white)](panel/package.json)
[![CI](https://img.shields.io/github/actions/workflow/status/Hesper-Labs/leakshield/ci.yml?branch=main&label=CI)](.github/workflows/ci.yml)
[![CodeQL](https://img.shields.io/github/actions/workflow/status/Hesper-Labs/leakshield/codeql.yml?branch=main&label=CodeQL)](.github/workflows/codeql.yml)
[![OpenSSF Scorecard](https://img.shields.io/badge/OpenSSF%20Scorecard-pending-lightgrey.svg)](https://securityscorecards.dev/)
[![OpenSSF Best Practices](https://img.shields.io/badge/OpenSSF%20Best%20Practices-pending-lightgrey.svg)](https://www.bestpractices.dev/)

</div>

<img src="https://raw.githubusercontent.com/Hesper-Labs/leakshield/main/assets/banner.png" alt="LeakShield architecture banner" />

LeakShield is an open-source **AI Gateway + DLP** that sits between your employees and the LLM
providers they use (OpenAI, Anthropic, Google, Azure). It isolates provider keys, inspects every
prompt with a **local LLM of your choice** for sensitive data leaks, and gives you full
per-employee audit and usage analytics.

## Why LeakShield

When employees use ChatGPT, Claude, or any LLM through your company's API key, they routinely
paste content that should never leave the building — customer records, identity numbers, contract
text, internal financials. That data ends up in provider logs and may end up in training pipelines.
Most existing tools fix only one half of this problem:

- DLP libraries (Llama Guard, NeMo, Presidio) classify content but don't manage keys, billing, or
  routing.
- AI gateways (LiteLLM, Portkey) manage keys and routing but ship no real DLP.
- Cloud DLP services (Lakera, Protect AI) work, but they're SaaS — your prompts leave your
  network.

LeakShield does both, locally. Your prompts never leave your infrastructure unless they pass DLP.

> **Self-host only.** LeakShield is designed to run inside your network. There is no managed
> SaaS offering and no telemetry phone-home. The only data the project operators see is whatever
> you choose to file in a public GitHub issue.

## Features

- **Multi-protocol native gateway** — OpenAI, Anthropic, Google Gemini, and Azure OpenAI exposed
  on their own native endpoints (`/openai/v1/*`, `/anthropic/v1/*`, etc.). Existing SDKs and CLIs
  (OpenAI Python, Anthropic Python, **Claude Code CLI**, Cursor, Aider, Continue.dev) work by just
  changing the `base_url`.
- **Per-employee virtual keys** — issue, revoke, rate-limit, and budget-cap keys per user.
  Master provider keys are stored encrypted under envelope encryption (KEK ⊃ DEK).
- **Pluggable local DLP** — pick your own model and your own strategy from the admin UI:
  - Specialized DLP classifiers (Llama Guard 3, ShieldGemma, etc.) — optional convenience
  - **Any general LLM as a judge** with a custom, admin-editable prompt (Llama 3.2, Qwen 2.5,
    Mistral, Phi — your call)
  - Hybrid: Microsoft Presidio (regex/NER, including Turkish-aware recognizers like TC kimlik,
    IBAN, GSM) escalating ambiguous content to your chosen LLM
- **Custom DLP policies** — edit the judge prompt in a Monaco-powered editor with a built-in
  test harness. An adversarial test suite gates deploys so a malicious admin can't ship an
  "always allow" prompt.
- **Multi-tenant** — many companies on one deploy, with PostgreSQL row-level security for hard
  tenant isolation.
- **Production-ready streaming** — SSE end-to-end with HTTP/2 multiplexing to providers.
  Optional output-side filtering for response-side leak prevention.
- **Full audit + analytics** — per-user requests, tokens, cost, blocked categories, latency
  percentiles. Live audit log via SSE. Tamper-evident hash chain on every record.
- **No model auto-download** — `docker compose up` works out of the box without pulling any LLM
  weights. The default inspector backend is a mock filter so you can wire up the gateway end-to-end
  before deciding which model you want.
- **On-premise first** — Docker Compose for dev/single-node, Helm chart for Kubernetes. KEK from
  Vault, AWS KMS, GCP KMS, or Azure Key Vault.

## Quick Start

```bash
git clone https://github.com/Hesper-Labs/leakshield
cd leakshield
docker compose up -d
# Panel at http://localhost:3000 → setup wizard walks you through:
#   1. Admin account
#   2. First provider key (OpenAI/Anthropic/Google/Azure)
#   3. First virtual key
#   4. DLP strategy + (optionally) which local LLM to use
#   5. Live test request
```

To enable a real local LLM (Ollama-backed) instead of the mock filter:

```bash
docker compose --profile local-llm up -d
# Then in the panel: Settings → DLP → Backend → Ollama
# Pick any model you've already pulled: `ollama pull qwen2.5:3b`, etc.
# LeakShield does NOT pull models for you — the choice and disk space are yours.
```

For Kubernetes, see [`deploy/helm/leakshield/`](deploy/helm/leakshield/README.md).

## Architecture

```mermaid
flowchart LR
  subgraph Clients["Clients (existing SDKs / CLIs)"]
    OAI[OpenAI SDK]
    ANT[Anthropic SDK / Claude Code]
    GEM[Gemini SDK]
    AZ[Azure SDK / Cursor / Aider]
  end

  subgraph LeakShield["LeakShield (self-hosted)"]
    direction LR
    GW["Gateway (Go)\n /openai · /anthropic\n /google · /azure\n /v1 · /admin"]
    INS["Inspector (Python)\n DLP strategies\n mock · hybrid · specialized · judge"]
    LLM["Your local LLM\n Ollama · vLLM · llama.cpp\n openai_compat"]
    PG[(PostgreSQL)]
    RD[(Redis)]
  end

  subgraph Providers["Upstream LLM providers"]
    P1[OpenAI]
    P2[Anthropic]
    P3[Google Gemini]
    P4[Azure OpenAI]
  end

  Clients -->|virtual key| GW
  GW <-->|gRPC InspectPrompt| INS
  INS <-->|HTTP / OpenAI-compat| LLM
  GW --- PG
  GW --- RD
  GW -->|allow| Providers
  GW -.->|block / mask| Clients
```

Detailed design: [docs/architecture.md](docs/architecture.md). Full HTTP contract:
[docs/openapi.yaml](docs/openapi.yaml) (with a curl walkthrough in [docs/api.md](docs/api.md)).

## Comparison

LeakShield is the intersection of "AI gateway" and "DLP", entirely self-hosted. Quick differentiators
versus adjacent tools:

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

The point of the table is not "every cell is yes for us"; it's that LeakShield is the only project
combining multi-protocol gateway, encrypted key custody, custom LLM-as-judge DLP, and tenant-aware
audit in one self-hosted package.

## Client Examples

### Claude Code CLI (Anthropic native)

```bash
export ANTHROPIC_BASE_URL=http://leakshield.example.com/anthropic
export ANTHROPIC_API_KEY=gw_live_xxxxxxxxxxxxxxxxxxxx
claude
```

### OpenAI Python SDK

```python
from openai import OpenAI
client = OpenAI(
    base_url="http://leakshield.example.com/openai/v1",
    api_key="gw_live_xxxxxxxxxxxxxxxxxxxx",
)
client.chat.completions.create(model="gpt-4o-mini", messages=[...])
```

### Anthropic Python SDK

```python
from anthropic import Anthropic
client = Anthropic(
    base_url="http://leakshield.example.com/anthropic",
    api_key="gw_live_xxxxxxxxxxxxxxxxxxxx",
)
client.messages.create(model="claude-sonnet-4-6", messages=[...])
```

More: [examples/](examples/).

## Repository Layout

| Directory | Contents |
|---|---|
| [`gateway/`](gateway/) | Go binary — proxy, admin API, worker |
| [`inspector/`](inspector/) | Python package — gRPC inspector + DLP strategies |
| [`panel/`](panel/) | Next.js admin panel |
| [`proto/`](proto/) | Inspector gRPC contract |
| [`deploy/`](deploy/) | Helm chart + production docker-compose |
| [`docs/`](docs/) | Architecture, security, deployment, provider guides, OpenAPI |
| [`examples/`](examples/) | Client SDK / CLI examples |
| [`assets/`](assets/) | Logo + banner art |
| [`.github/`](.github/) | CI / CodeQL / release workflows, issue templates, CODEOWNERS |

## Status & Roadmap

**Pre-alpha** — under active development. Target for v1.0 is full-scope production-ready: four
provider adapters, three DLP strategies, streaming, admin panel, analytics, audit, KMS-backed
encryption, Helm chart.

- Track planned work on the public GitHub
  [milestones](https://github.com/Hesper-Labs/leakshield/milestones).
- Read the [unreleased changelog](CHANGELOG.md#unreleased) for a checkpoint of what has shipped on
  `main`.
- Vulnerability disclosures and supported versions: [SECURITY.md](SECURITY.md).

## Contributors

<!-- BEGIN: all-contributors-bot placeholder -->
<!--
This section is maintained by the all-contributors bot once it's installed on the repo.
Until then, contributors are listed in [MAINTAINERS.md](MAINTAINERS.md) and recognised in
release notes. To add yourself, comment on any open issue or PR with
`@all-contributors please add @<your-handle> for <type>`.
-->
<table>
  <tr>
    <td align="center" colspan="6">
      <em>Contributor list will appear here once the all-contributors bot is enabled.</em>
    </td>
  </tr>
</table>
<!-- END: all-contributors-bot placeholder -->

## Show Your Support

If LeakShield is useful to you:

- [![GitHub stars](https://img.shields.io/github/stars/Hesper-Labs/leakshield?style=social)](https://github.com/Hesper-Labs/leakshield/stargazers) — give the repo a star.
- Share on [Twitter / X](https://twitter.com/intent/tweet?text=LeakShield%20%E2%80%94%20open-source%20AI%20gateway%20%2B%20DLP%20you%20can%20self-host&url=https%3A%2F%2Fgithub.com%2FHesper-Labs%2Fleakshield), [Bluesky](https://bsky.app/intent/compose?text=LeakShield%20%E2%80%94%20open-source%20AI%20gateway%20%2B%20DLP%20you%20can%20self-host%20https%3A%2F%2Fgithub.com%2FHesper-Labs%2Fleakshield), [LinkedIn](https://www.linkedin.com/sharing/share-offsite/?url=https%3A%2F%2Fgithub.com%2FHesper-Labs%2Fleakshield), or [Hacker News](https://news.ycombinator.com/submitlink?u=https%3A%2F%2Fgithub.com%2FHesper-Labs%2Fleakshield&t=LeakShield%20%E2%80%94%20open-source%20AI%20gateway%20%2B%20DLP).
- Open an issue with your use case so we know what to prioritise.

## License

[Apache 2.0](LICENSE).

## Contributing & Security

- [CONTRIBUTING.md](CONTRIBUTING.md) — development setup, testing, PR conventions.
- [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) — Contributor Covenant 2.1.
- [SECURITY.md](SECURITY.md) — how to report a vulnerability.
