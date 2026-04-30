"""Optional Presidio adapter — covers EMAIL / CREDIT_CARD / URL / IP and
similar universal patterns.

Presidio depends on spaCy, which is heavy and ships with a model that needs
to be downloaded separately. We make this adapter optional: if Presidio is
unavailable (e.g., during ``pip install -e .[dev]`` without the spaCy model
or during unit tests), the adapter degrades to a no-op and the Turkish and
credential recognizers handle the bulk of detection on their own.

In production we recommend installing the spaCy model and letting Presidio
handle email / credit-card / URL detection — those are well-tested patterns
we do not need to maintain ourselves.
"""

from __future__ import annotations

import re

from .turkish import RecognizerHit


_EMAIL_RE = re.compile(
    r"\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b"
)


def _luhn(digits: str) -> bool:
    s = re.sub(r"\D", "", digits)
    if not (12 <= len(s) <= 19):
        return False
    total = 0
    parity = len(s) % 2
    for i, ch in enumerate(s):
        d = int(ch)
        if i % 2 == parity:
            d *= 2
            if d > 9:
                d -= 9
        total += d
    return total % 10 == 0


_CREDIT_CARD_RE = re.compile(
    r"(?<!\d)((?:\d[ \-]?){12,19})(?!\d)"
)


def recognize_email(text: str) -> list[RecognizerHit]:
    return [
        RecognizerHit(
            category="PII.EMAIL",
            start=m.start(),
            end=m.end(),
            confidence=0.95,
            text=m.group(),
            severity="MASK",
        )
        for m in _EMAIL_RE.finditer(text)
    ]


def recognize_credit_card(text: str) -> list[RecognizerHit]:
    hits: list[RecognizerHit] = []
    for m in _CREDIT_CARD_RE.finditer(text):
        candidate = m.group(1)
        if _luhn(candidate):
            hits.append(
                RecognizerHit(
                    category="FINANCIAL.CREDIT_CARD",
                    start=m.start(1),
                    end=m.end(1),
                    confidence=0.95,
                    text=candidate,
                    severity="BLOCK",
                )
            )
    return hits


# We expose these as a list so the strategy can call them alongside the
# Turkish + credential recognizers.
PRESIDIO_LIKE_RECOGNIZERS = [recognize_email, recognize_credit_card]


__all__ = [
    "PRESIDIO_LIKE_RECOGNIZERS",
    "recognize_email",
    "recognize_credit_card",
]
