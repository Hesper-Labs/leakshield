"""DLP filter strategies.

Built-in:
  - ``mock``        — always ALLOW; dev / smoke-test default.
  - ``hybrid``      — Presidio (regex/NER) + escalation to a user-chosen LLM.
  - ``specialized`` — purpose-trained DLP classifier (Llama Guard, ShieldGemma...).
  - ``judge``       — any general LLM driven by an admin-editable prompt.

Adding a strategy: drop a module in this package, subclass :class:`Filter`,
and register it in :func:`leakshield_inspector.server.get_filter`.
"""

from .base import Category, Decision, Filter, Message, Span, Verdict

__all__ = ["Category", "Decision", "Filter", "Message", "Span", "Verdict"]
