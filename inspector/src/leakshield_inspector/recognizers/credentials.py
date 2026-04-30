"""Credential / secret recognizers used by the Hybrid strategy.

Patterns are tuned to err on the side of recall for real secrets while
avoiding obvious false positives (placeholder-looking strings such as
``sk-XXXXXXX``). The Hybrid strategy combines these hits with the Turkish
recognizers and the company-custom categories before deciding.
"""

from __future__ import annotations

import math
import re
from collections.abc import Callable

from .turkish import RecognizerHit


# Anthropic keys must be matched first because their prefix overlaps the
# generic OpenAI prefix and we do not want to double-flag.
_ANTHROPIC_RE = re.compile(r"\bsk-ant-[A-Za-z0-9_\-]{20,}\b")
_OPENAI_RE = re.compile(r"\bsk-(?!ant-)[A-Za-z0-9_\-]{20,}\b")
_AWS_ACCESS_KEY_RE = re.compile(r"\b(AKIA[0-9A-Z]{16})\b")
_PEM_RE = re.compile(
    r"-----BEGIN (?:RSA |EC |DSA |OPENSSH |ENCRYPTED |PGP |)PRIVATE KEY-----"
)


def recognize_anthropic_key(text: str) -> list[RecognizerHit]:
    return [
        RecognizerHit(
            category="CREDENTIAL.ANTHROPIC_KEY",
            start=m.start(),
            end=m.end(),
            confidence=0.99,
            text=m.group(),
            severity="BLOCK",
        )
        for m in _ANTHROPIC_RE.finditer(text)
    ]


def recognize_openai_key(text: str) -> list[RecognizerHit]:
    return [
        RecognizerHit(
            category="CREDENTIAL.OPENAI_KEY",
            start=m.start(),
            end=m.end(),
            confidence=0.95,
            text=m.group(),
            severity="BLOCK",
        )
        for m in _OPENAI_RE.finditer(text)
    ]


def recognize_aws_access_key(text: str) -> list[RecognizerHit]:
    return [
        RecognizerHit(
            category="CREDENTIAL.AWS_ACCESS_KEY",
            start=m.start(1),
            end=m.end(1),
            confidence=0.99,
            text=m.group(1),
            severity="BLOCK",
        )
        for m in _AWS_ACCESS_KEY_RE.finditer(text)
    ]


def recognize_pem(text: str) -> list[RecognizerHit]:
    hits: list[RecognizerHit] = []
    for m in _PEM_RE.finditer(text):
        hits.append(
            RecognizerHit(
                category="CREDENTIAL.PRIVATE_KEY",
                start=m.start(),
                end=m.end(),
                confidence=1.0,
                text=m.group(),
                severity="BLOCK",
            )
        )
    return hits


# ---------------------------------------------------------------------------
# Generic high-entropy token near a "secret-like" keyword.
# ---------------------------------------------------------------------------
_GENERIC_TOKEN_KEYWORD_RE = re.compile(
    r"""
    \b(api[_\-]?key|token|secret|bearer|authorization|password|passwd|pwd)
    \s*[:=]\s*
    ['"]?
    ([A-Za-z0-9_\-\.+/=]{20,})
    ['"]?
    """,
    re.IGNORECASE | re.VERBOSE,
)


def _shannon_entropy(s: str) -> float:
    """Bits-per-character entropy. Random 64-char strings hit ~5–6 bits."""
    if not s:
        return 0.0
    freqs: dict[str, int] = {}
    for ch in s:
        freqs[ch] = freqs.get(ch, 0) + 1
    n = len(s)
    return -sum((c / n) * math.log2(c / n) for c in freqs.values())


def recognize_generic_secret(text: str) -> list[RecognizerHit]:
    """Match ``api_key=...``, ``token: ...`` etc. when the value is high-entropy.

    We require a minimum entropy of 3.5 bits/char; ``placeholder`` and
    ``my-api-key-here`` fall well below that bar.
    """
    hits: list[RecognizerHit] = []
    for m in _GENERIC_TOKEN_KEYWORD_RE.finditer(text):
        token = m.group(2)
        if _shannon_entropy(token) < 3.5:
            continue
        # Skip obvious placeholders.
        if token.lower() in {"placeholder", "your-key-here", "xxxxxxxxxxxx"}:
            continue
        hits.append(
            RecognizerHit(
                category="CREDENTIAL.GENERIC_API_KEY",
                start=m.start(2),
                end=m.end(2),
                confidence=0.7,
                text=token,
                severity="BLOCK",
            )
        )
    return hits


CREDENTIAL_RECOGNIZERS: list[Callable[[str], list[RecognizerHit]]] = [
    recognize_anthropic_key,  # before openai (more specific prefix)
    recognize_openai_key,
    recognize_aws_access_key,
    recognize_pem,
    recognize_generic_secret,
]
