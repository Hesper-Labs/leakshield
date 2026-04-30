"""DLP filter strategies.

Built-in:
  - ``mock``        — always ALLOW; dev / smoke-test default.
  - ``hybrid``      — Presidio (regex/NER) + escalation to a user-chosen LLM.
  - ``specialized`` — purpose-trained DLP classifier (Llama Guard, ShieldGemma...).
  - ``judge``       — any general LLM driven by an admin-editable prompt.

All strategies (apart from ``mock``) consume two category sources:

1. **Built-in** universal recognizers (PII, credentials, source-code-embedded secrets).
2. **Company-custom** categories defined per tenant via keyword lists, regex patterns,
   document fingerprints, hashed directories, or LLM-only category descriptions. These
   describe content that would harm the specific company — proprietary project names,
   customer lists, contract language, M&A discussions, internal financials, etc.

See ``docs/dlp-categories.md`` for the full taxonomy and authoring model.

Adding a strategy: drop a module in this package, subclass :class:`Filter`,
and register it in :func:`leakshield_inspector.server.get_filter`.
"""

from .base import Category, Decision, Filter, Message, Span, Verdict

__all__ = ["Category", "Decision", "Filter", "Message", "Span", "Verdict"]
