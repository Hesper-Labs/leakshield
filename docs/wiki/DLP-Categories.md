# DLP Categories

LeakShield separates two layers of DLP detection:

1. **Built-in categories** — universal patterns that every deployment benefits from (PII,
   credentials, financial identifiers).
2. **Company-custom categories** — anything *that specific company* needs to protect: proprietary
   project names, customer lists, internal financials, contract language, M&A discussions,
   source code with embedded secrets, and so on.

The whole point of the inspector is that DLP is rarely "just PII." A pharma company cares about
trial code names, a fintech cares about transaction detail, a law firm cares about client
identity, a startup cares about pre-announcement product names. LeakShield ships sane defaults
for the universal half and lets the admin describe the company-specific half through several
mechanisms layered on top of each other.

## Built-in categories

The Hybrid strategy ships these recognizers out of the box. They run as a fast Presidio path
before the LLM is consulted.

| Category | Examples |
|---|---|
| `PII.NAME` | Proper-name detection |
| `PII.EMAIL` | RFC 5322 email addresses |
| `PII.PHONE` | International + Turkish GSM formats |
| `PII.TC_KIMLIK` | Turkish national ID with checksum validation |
| `PII.IBAN` | IBAN with MOD-97 validation |
| `PII.PASSPORT` | Common passport number formats |
| `PII.DOB` | Dates of birth |
| `PII.ADDRESS` | Street addresses (heuristic) |
| `FINANCIAL.CREDIT_CARD` | Luhn-validated card numbers |
| `FINANCIAL.IBAN` | (alias of PII.IBAN; surfaces in finance dashboards) |
| `FINANCIAL.AMOUNT_TL` | Turkish lira amounts above a threshold |
| `CREDENTIAL.OPENAI_KEY` | `sk-...` keys |
| `CREDENTIAL.ANTHROPIC_KEY` | `sk-ant-...` keys |
| `CREDENTIAL.GENERIC_API_KEY` | High-entropy strings near keywords like "api", "token" |
| `CREDENTIAL.AWS_ACCESS_KEY` | `AKIA...` patterns |
| `CREDENTIAL.PRIVATE_KEY` | PEM headers (`-----BEGIN ... PRIVATE KEY-----`) |
| `CODE.SECRET_IN_SOURCE` | Source-code blocks containing the above credentials |

Each recognizer reports a confidence score. The Hybrid strategy treats high-confidence hits as
direct BLOCK, medium-confidence as ESCALATE-to-LLM, and low-confidence as ALLOW.

## Company-custom categories

Admins define custom categories from the panel (Policy → Categories). A category can use any
combination of the following mechanisms, all of which feed into both the Hybrid strategy and the
LLM Judge prompt.

### 1. Keyword lists

Plain strings, case-insensitive by default, with optional whole-word matching:

```yaml
- name: PROJECT.BLUEMOON
  description: "Internal codename for the next-gen platform"
  severity: BLOCK
  keywords:
    - "Project Bluemoon"
    - "Bluemoon initiative"
    - "PBM-"            # ticket prefix
```

### 2. Regex patterns

For structured identifiers:

```yaml
- name: INTERNAL.TICKET_ID
  description: "Jira ticket IDs from the private project"
  severity: MASK
  regex:
    - 'ACME-\d{4,6}'
    - 'INT-[A-Z]{3}-\d+'
```

### 3. Document fingerprints

Marker strings that identify a document class. When seen, the entire prompt is treated as the
classified document type and BLOCK / MASK is applied based on severity.

```yaml
- name: DOC.CONFIDENTIAL
  description: "Documents marked confidential at the top"
  severity: BLOCK
  fingerprints:
    - "Confidential — Internal Only"
    - "DRAFT: NOT FOR DISTRIBUTION"
    - "©2026 Acme Corp — All rights reserved"
```

### 4. LLM judge category descriptions

For categories that no static rule can express ("any discussion of pending M&A activity"), the
admin writes a short description that gets injected into the judge prompt:

```yaml
- name: BUSINESS.MNA
  description: |
    Any mention of pending mergers, acquisitions, divestitures, or due-diligence
    activity. Includes target company names, valuation figures, advisor names,
    or scheduling of related meetings.
  severity: BLOCK
  llm_only: true       # only consulted via the judge
```

### 5. Customer / employee directories

Bulk lists of names treated as PII for the company's specific universe:

```yaml
- name: CUSTOMER.NAME
  description: "Active customer accounts"
  severity: MASK
  source: customer_directory   # uploaded CSV, hashed for storage
```

The directory is stored hashed; the inspector uses a Bloom filter at evaluation time so the raw
list never has to be loaded into LLM context.

## Severities

| Severity | Effect |
|---|---|
| `ALLOW` | Logged but not blocked (useful for analytics on sensitive-but-permitted topics) |
| `MASK` | Replace matching spans with `[REDACTED:CATEGORY]` and forward the masked prompt |
| `BLOCK` | Reject the request with a 403 and a structured error |

A category may declare different severities per environment (e.g., MASK in dev, BLOCK in prod).

## How the strategies combine these

- **Mock**: ignores all categories, always ALLOW. Default at first install.
- **Hybrid (recommended)**: runs all built-in recognizers + every company recognizer with a
  pattern (keyword / regex / fingerprint / directory). Hits at high confidence apply their
  severity directly. Low or ambiguous hits, or any prompt mentioning an `llm_only` category,
  ESCALATE to the configured LLM with the relevant category descriptions in the judge prompt.
- **Specialized**: a DLP-trained model handles built-in categories; company-custom categories
  still escalate to the model via the same prompt slot.
- **Judge**: the entire prompt is sent to the chosen LLM with the full category list (built-in +
  custom) embedded in the judge scaffold.

## Authoring custom categories from the panel

The Policy → Categories screen exposes:

- Built-in catalog (read-only) so the admin sees what's already covered.
- "Add custom category" wizard that walks through name / description / severity / mechanism.
- A Monaco-powered YAML editor for power users.
- Test harness: paste a sample prompt, see exactly which built-in and custom categories fire,
  with confidence scores and the spans that triggered them.
- Versioning + adversarial test gate: a new category that would have allowed a previously
  blocked sample is rejected at save time.

## Privacy of the company's own DLP rules

Custom category content (keyword lists, regex, directory entries) is itself sensitive — it
encodes what the company considers secret. It's stored encrypted under the company DEK, just
like master provider keys, and never logged or sent in OpenTelemetry traces.
