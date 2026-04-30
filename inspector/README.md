# LeakShield Inspector

Inspector is the DLP decision component of the gateway. It runs as a gRPC service; the gateway
calls it for every request, sends the prompt, and receives an `ALLOW / BLOCK / MASK / ESCALATE`
verdict.

## Strategies

The strategy is chosen per-tenant from the admin UI. All three are pluggable; you can also write
your own.

| Strategy | Status | Description |
|---|---|---|
| `mock` | ✅ | Always returns ALLOW. Default in `docker compose up` so the gateway works end-to-end without any local LLM. |
| `hybrid` | 🚧 | Microsoft Presidio (regex / NER, including Turkish recognizers for TC kimlik, IBAN, GSM, e-mail, credit cards) for fast detection, with escalation to your chosen LLM for ambiguous content. Recommended default once a model is available. |
| `specialized` | 🚧 | Use a model trained for DLP: Llama Guard 3 (1B / 8B), ShieldGemma, or similar. Optional shortcut. |
| `judge` | 🚧 | Use **any** general-purpose LLM as a judge, driven by an admin-editable prompt. The judge prompt has an immutable scaffold and an adversarial test suite gating deploys. Works with Llama 3.2, Qwen 2.5, Mistral, Phi — anything you can serve locally. |

LeakShield does **not** force you onto a specific model. You pick the model and the backend; we
just integrate.

## Backends

| Backend | Status | Notes |
|---|---|---|
| `mock` | ✅ | No LLM. Always ALLOW. Useful for getting the rest of the stack running. |
| `ollama` | 🚧 | Talks to a local Ollama HTTP API. Works with any model you've pulled. |
| `vllm` | 🚧 | Higher throughput on GPU; OpenAI-compatible HTTP server. |
| `llamacpp` | 🚧 | Embedded `llama-cpp-python`; runs in-process. |
| `openai_compat` | 🚧 | Any local OpenAI-compatible endpoint (LM Studio, text-generation-webui, etc.). |

LeakShield never downloads models on your behalf — that's your decision and your disk.

## Running locally

```bash
pip install -e '.[dev]'
python -m leakshield_inspector
```

## Tests

```bash
pytest
```
