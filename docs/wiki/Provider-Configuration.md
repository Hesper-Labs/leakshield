# Provider Configuration

Each provider has a slightly different shape for the connection details. The panel's
"Connect a provider" wizard handles all four; this page documents what each one needs and what
goes into the encrypted `config` blob.

## OpenAI

| Field | Required | Notes |
|---|---|---|
| `api_key` | yes | `sk-...` (project key) or org-key. Encrypted under the tenant DEK. |
| `base_url` | no | Override the default `https://api.openai.com/v1`. Useful if you front OpenAI through your own proxy or a VPC peering setup. |
| `org_id` | no | Sent as `OpenAI-Organization` header. |

Test-connection probe: `GET /v1/models` with the supplied key. Success returns the visible model
list, populated into the panel as the "allowed models" preview.

## Anthropic

| Field | Required | Notes |
|---|---|---|
| `api_key` | yes | `sk-ant-...` |
| `base_url` | no | Override the default `https://api.anthropic.com`. |

Test-connection probe: `POST /v1/messages` with `max_tokens: 1` and `model: claude-haiku-4-5-20251001`.
Anthropic does not expose a free model-listing endpoint, so the panel populates the model list
with the public catalog (Opus / Sonnet / Haiku) when the probe succeeds.

## Google Gemini

| Field | Required | Notes |
|---|---|---|
| `api_key` | yes | A Google AI Studio key. |
| `project_id` | yes | Required for billing attribution and for some of the v1beta endpoints. |

Test-connection probe: `GET /v1beta/models?key=…`. The gateway accepts requests using either
`?key=…` (the user's virtual key, swapped on the way out) or `Authorization: Bearer …`.

## Azure OpenAI

Azure is the most config-heavy because of its deployment model.

| Field | Required | Notes |
|---|---|---|
| `api_key` | yes | The resource's key1 / key2. |
| `endpoint` | yes | e.g. `https://my-resource.openai.azure.com` |
| `api_version` | yes | Default `2024-08-01-preview`. |
| `deployments` | yes | Map of `model → deployment_name`, e.g. `{ "gpt-4o": "gpt4o-prod" }`. |

Two URL shapes are accepted on the proxy:

1. The client already chose a deployment:
   `POST /azure/openai/deployments/<deployment>/chat/completions?api-version=...`
2. The client uses a placeholder `-` and relies on `body.model` to look up the deployment in
   `deployments`:
   `POST /azure/openai/deployments/-/chat/completions?api-version=...`

Shape 2 lets OpenAI-flavoured SDKs (which only know about model names, not Azure deployments)
work without any code changes.

## Where the config goes

The plaintext API key never lives in the database. On `POST /admin/v1/providers`:

1. The panel proxies the request to the gateway with the bearer token.
2. The gateway encrypts `api_key` under the tenant DEK and stores `api_key_cipher` +
   `api_key_nonce` in `master_provider_keys`.
3. The non-secret fields (`base_url`, `org_id`, `endpoint`, `api_version`, `deployments`) go into
   `config JSONB` on the same row.
4. On every proxy request, the gateway looks up the active row for that tenant + provider,
   unwraps the DEK (cached 15 m), decrypts the API key, and uses it for the upstream call.

## Allowlists

Each row in `master_provider_keys` carries an optional `allowed_models` array. If non-empty,
requests to a model not on the list 403 with `model_not_allowed`. The panel populates this from
the test-connection probe and lets the admin trim the set.

Per-virtual-key allowlists override the provider-level one — see
[Virtual Keys](Virtual-Keys#allowlists-and-budgets).

## Rotation

Adding a fresh key for the same provider creates a new active row and (on the roadmap) marks the
previous row `is_active = false` after a 24-hour grace period so in-flight requests don't 401.
For now, rotation is "create new, then DELETE old via `DELETE /admin/v1/providers/{id}`".
