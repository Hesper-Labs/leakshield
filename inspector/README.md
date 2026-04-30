# LeakShield Inspector

Inspector is the DLP decision component of the gateway. It runs as a gRPC service; the gateway
calls it for every request, sends the prompt, and receives an `ALLOW / BLOCK / MASK / ESCALATE`
verdict.

## Strategies

The strategy is chosen per-tenant from the admin UI. All strategies are pluggable; you can also
write your own.

| Strategy | Status | Description |
|---|---|---|
| `mock` | OK | Always returns ALLOW. Default in `docker compose up` so the gateway works end-to-end without any local LLM. |
| `hybrid` | OK | Built-in PII recognizers (Turkish-aware: TC kimlik, IBAN, GSM, address) + credential detection (OpenAI / Anthropic / AWS / generic high-entropy / PEM private keys) + company-custom categories (keyword / regex / fingerprint / hashed-directory) with optional escalation to a configured LLM judge. |
| `specialized` | TODO | Use a model trained for DLP: Llama Guard 3 (1B / 8B), ShieldGemma, or similar. Optional shortcut. |
| `judge` | TODO | Use **any** general-purpose LLM as a judge, driven by an admin-editable prompt. Works with Llama 3.2, Qwen 2.5, Mistral, Phi — anything you can serve locally. |

LeakShield does **not** force you onto a specific model. You pick the model and the backend; we
just integrate.

## Backends

| Backend | Status | Notes |
|---|---|---|
| `mock` | OK | No LLM. Always ALLOW. Useful for getting the rest of the stack running. |
| `ollama` | OK | Talks to a local Ollama HTTP API (`/api/chat`). Works with any model you've pulled. Honors JSON-Schema-constrained sampling on Ollama >= 0.5; falls back to `format=json` on older releases. |
| `vllm` | TODO | Higher throughput on GPU; OpenAI-compatible HTTP server. Stub raises NotImplementedError. |
| `llamacpp` | TODO | Embedded `llama-cpp-python`; runs in-process. Stub raises NotImplementedError. |
| `openai_compat` | TODO | Any local OpenAI-compatible endpoint (LM Studio, text-generation-webui, LiteLLM). Stub raises NotImplementedError. |

LeakShield never downloads models on your behalf — that's your decision and your disk.

## Built-in recognizers

Shipped with the `hybrid` strategy:

- `PII.TC_KIMLIK` — Turkish national ID with checksum validation.
- `PII.IBAN` — IBAN with MOD-97 validation.
- `PII.PHONE` — Turkish GSM (and ITU-prefixed) numbers.
- `PII.ADDRESS` — Turkish address heuristic (Mah./Cad./Sok./No: combinations).
- `CREDENTIAL.OPENAI_KEY` — `sk-...` keys (excludes Anthropic prefix).
- `CREDENTIAL.ANTHROPIC_KEY` — `sk-ant-...` keys.
- `CREDENTIAL.AWS_ACCESS_KEY` — `AKIA...` access key IDs.
- `CREDENTIAL.GENERIC_API_KEY` — high-entropy tokens labelled `api_key=`, `token:`, `secret:`, etc.
- `CREDENTIAL.PRIVATE_KEY` — PEM `-----BEGIN PRIVATE KEY-----` headers.

## Adding a custom category

Custom categories are sent through the policy config blob (the JSON the gateway forwards in
`InspectRequest.config_blob`):

```python
from leakshield_inspector.categories import hash_directory_entry

# 1. Hash directory entries on the panel side (mirrors what the inspector expects).
employees = ["Ayşe Yılmaz", "Mehmet Demir"]
employee_hashes = [hash_directory_entry(n).hex() for n in employees]

# 2. Define categories.
custom_categories = [
    {
        "name": "PROJECT.BLUEMOON",
        "severity": "BLOCK",
        "description": "Internal codename for the next-gen platform",
        "keywords": ["Project Bluemoon", "Bluemoon initiative"],
    },
    {
        "name": "INTERNAL.TICKET_ID",
        "severity": "MASK",
        "regex": [r"ACME-\d{4,6}"],
    },
    {
        "name": "DOC.CONFIDENTIAL",
        "severity": "BLOCK",
        "fingerprints": ["Confidential — Internal Only"],
    },
    {
        "name": "EMPLOYEE.NAME",
        "severity": "MASK",
        "directory_hashes": employee_hashes,
    },
    {
        "name": "BUSINESS.MNA",
        "severity": "BLOCK",
        "description": "any pending merger / acquisition / divestiture",
        "llm_only": True,  # consulted only by the LLM judge
    },
]
```

The four mechanisms are evaluated in this order:

1. **Fingerprints** — substring match; if hit, the entire prompt is classified as the category type.
2. **Keywords** — case-insensitive substring (with optional whole-word) matching; positions tracked.
3. **Regex** — compiled Python regex; positions tracked.
4. **Directory hashes** — 16-byte truncated SHA-256 of normalized names. The inspector keeps a Bloom
   filter (96k bits, 7 hashes, sized for 10k entries at 1% FPR) for cheap negative checks and
   verifies positives against an exact hash set.

`llm_only` categories skip the rule layer and are pushed into the judge prompt.

## Configuration

| Variable | Default | Notes |
|---|---|---|
| `LEAKSHIELD_INSPECTOR_PORT` | `50051` | gRPC port |
| `LEAKSHIELD_INSPECTOR_BIND` | `0.0.0.0` | bind address |
| `LEAKSHIELD_INSPECTOR_BACKEND` | `mock` | `mock` / `ollama` / `vllm` / `llamacpp` / `openai_compat` |
| `LEAKSHIELD_INSPECTOR_BACKEND_URL` | `http://localhost:11434` | backend HTTP URL (Ollama / vLLM / OpenAI-compat) |
| `LEAKSHIELD_INSPECTOR_DEFAULT_STRATEGY` | `mock` | strategy when the per-tenant policy doesn't pick one |
| `LEAKSHIELD_INSPECTOR_JUDGE_MODEL` | `qwen2.5:3b-instruct` | model used by the hybrid judge |
| `LEAKSHIELD_INSPECTOR_MAX_INFLIGHT` | `32` | concurrent inspector calls (semaphore) |

## Regenerating protobuf stubs

The Python stubs in `src/leakshield_inspector/proto/` are assembled programmatically at import
time via `google.protobuf.descriptor_pb2`. There is no build-time `protoc` step needed for the
Python service. If you change `proto/inspector/v1/inspector.proto`, update
`inspector_pb2._build_file_descriptor()` to match. (See the header comment in
`src/leakshield_inspector/proto/__init__.py` for the rationale.)

If you prefer to regenerate from `protoc` directly (e.g., for cross-language stubs), run:

```bash
cd inspector
python -m grpc_tools.protoc \
    --python_out=src/leakshield_inspector/proto \
    --grpc_python_out=src/leakshield_inspector/proto \
    --pyi_out=src/leakshield_inspector/proto \
    --proto_path=../proto \
    ../proto/inspector/v1/inspector.proto
```

The generated files use `from inspector.v1 import inspector_pb2` style imports; you'll need to
adjust to match `leakshield_inspector.proto.inspector_pb2`.

## Running locally

```bash
pip install -e '.[dev]'
python -m leakshield_inspector
```

The server logs `inspector_ready address=0.0.0.0:50051` once it's ready to accept gRPC traffic.

## Tests

```bash
pytest -q
```

The suite is fully offline:

- `test_recognizers_turkish.py` — TC kimlik checksum, IBAN MOD-97, GSM, address heuristics.
- `test_recognizers_credentials.py` — API keys, AWS, PEM, generic high-entropy.
- `test_categories.py` — keyword / regex / fingerprint / directory matching, including overlap
  and span correctness.
- `test_hybrid_strategy.py` — end-to-end inspect calls with a scripted backend.
- `test_adversarial.py` — runs `tests/adversarial.jsonl` (30+ cases) to detect regressions.
- `test_server_smoke.py` — bootstraps the gRPC server on a free port and exercises Health,
  InspectPrompt, and InspectStreamWindow.
- `test_cache.py` — verdict cache get/set, key composition, cacheability rules.
