"""Turkish-aware PII recognizers.

These recognizers do strict structural validation (checksum / MOD-97) before
they fire so they do not trip on look-alike numbers (postal codes, customer
IDs, etc.). Confidence is reported per hit; the Hybrid strategy uses it to
decide BLOCK vs. ESCALATE.

Each recognizer is a plain callable. We do not use Presidio's ``Recognizer``
class directly because Presidio's analyzer pulls in ``spacy`` which we do
not want as a hard dependency for the unit tests. Instead the Hybrid
strategy combines these with the Presidio built-ins via a lightweight
adapter (see ``strategies/hybrid.py``).
"""

from __future__ import annotations

import re
from collections.abc import Callable
from dataclasses import dataclass


@dataclass
class RecognizerHit:
    """A single recognizer match.

    Attributes:
        category: dotted name like ``PII.TC_KIMLIK`` or ``CREDENTIAL.OPENAI_KEY``.
        start: char offset (inclusive).
        end: char offset (exclusive).
        confidence: 0.0–1.0; the Hybrid strategy uses low confidence to flag
            ESCALATE-to-LLM, but high-confidence hits respect ``severity``.
        text: the matched substring (used for redaction).
        severity: default action when this recognizer fires at high
            confidence. ``BLOCK`` for high-impact identifiers (national IDs,
            credentials), ``MASK`` for things we typically want to redact and
            keep flowing (phone, address).
    """

    category: str
    start: int
    end: int
    confidence: float
    text: str
    severity: str = "BLOCK"  # one of "BLOCK" | "MASK"


# ---------------------------------------------------------------------------
# TC Kimlik (Turkish national ID)
# ---------------------------------------------------------------------------
# Algorithm (legal definition):
#   * 11 digits.
#   * d1 != 0.
#   * d10 = ((d1+d3+d5+d7+d9) * 7 - (d2+d4+d6+d8)) mod 10.
#   * d11 = (d1+d2+...+d10) mod 10.
# We require word boundaries to avoid matching long numeric blobs.

_TC_KIMLIK_RE = re.compile(r"(?<!\d)(\d{11})(?!\d)")


def _is_valid_tc_kimlik(s: str) -> bool:
    if len(s) != 11 or not s.isdigit():
        return False
    digits = [int(c) for c in s]
    if digits[0] == 0:
        return False
    odd_sum = digits[0] + digits[2] + digits[4] + digits[6] + digits[8]
    even_sum = digits[1] + digits[3] + digits[5] + digits[7]
    d10 = (odd_sum * 7 - even_sum) % 10
    if d10 != digits[9]:
        return False
    d11 = sum(digits[:10]) % 10
    return d11 == digits[10]


def recognize_tc_kimlik(text: str) -> list[RecognizerHit]:
    hits: list[RecognizerHit] = []
    for m in _TC_KIMLIK_RE.finditer(text):
        if _is_valid_tc_kimlik(m.group(1)):
            hits.append(
                RecognizerHit(
                    category="PII.TC_KIMLIK",
                    start=m.start(1),
                    end=m.end(1),
                    confidence=0.99,
                    text=m.group(1),
                    severity="BLOCK",
                )
            )
    return hits


# ---------------------------------------------------------------------------
# IBAN (MOD-97 validation)
# ---------------------------------------------------------------------------
# We accept country-prefixed IBANs of length 15–34 chars. Allow optional
# spaces (every 4 chars per the spec).

_IBAN_RE = re.compile(
    r"(?<![A-Z0-9])([A-Z]{2}\d{2}(?:\s?[A-Z0-9]){11,30})(?![A-Z0-9])",
    re.IGNORECASE,
)


def _iban_mod97(raw: str) -> bool:
    s = re.sub(r"\s+", "", raw).upper()
    if not (15 <= len(s) <= 34):
        return False
    if not s[:2].isalpha() or not s[2:4].isdigit():
        return False
    # Move first 4 chars to end.
    rearranged = s[4:] + s[:4]
    # Convert each letter to a 2-digit number (A=10 .. Z=35).
    converted = []
    for ch in rearranged:
        if ch.isdigit():
            converted.append(ch)
        elif "A" <= ch <= "Z":
            converted.append(str(ord(ch) - 55))
        else:
            return False
    # Compute MOD-97 piecewise to keep the integer small.
    n = 0
    for c in "".join(converted):
        n = (n * 10 + int(c)) % 97
    return n == 1


def recognize_iban(text: str) -> list[RecognizerHit]:
    hits: list[RecognizerHit] = []
    for m in _IBAN_RE.finditer(text):
        raw = m.group(1)
        if _iban_mod97(raw):
            hits.append(
                RecognizerHit(
                    category="PII.IBAN",
                    start=m.start(1),
                    end=m.end(1),
                    confidence=0.99,
                    text=raw,
                    severity="BLOCK",
                )
            )
    return hits


# ---------------------------------------------------------------------------
# Turkish GSM (mobile) phone numbers
# ---------------------------------------------------------------------------
# Common formats:
#   +90 5XX XXX XX XX
#   0090 5XX...
#   0 5XX XXX XX XX
#   05XX XXX XX XX
#   5XX XXX XX XX  (ambiguous — only fire on exactly 10 digits starting with 5)
# Carrier codes start with 5 and the second digit is in {0..5} for current
# allocations (50, 53, 54, 55, but 51/52 also exist for newer MVNOs).

_GSM_RE = re.compile(
    r"""
    (?<![\w+])
    (
        (?:\+90|0090|90)?[\s\-\.]*0?[\s\-\.]*5\d{2}[\s\-\.]*\d{3}[\s\-\.]*\d{2}[\s\-\.]*\d{2}
    )
    (?![\w])
    """,
    re.VERBOSE,
)


def recognize_gsm(text: str) -> list[RecognizerHit]:
    hits: list[RecognizerHit] = []
    for m in _GSM_RE.finditer(text):
        raw = m.group(1)
        digits = re.sub(r"\D", "", raw)
        # Strip leading country code / trunk prefix.
        if digits.startswith("0090"):
            digits = digits[4:]
        elif digits.startswith("90"):
            digits = digits[2:]
        if digits.startswith("0"):
            digits = digits[1:]
        if len(digits) != 10 or not digits.startswith("5"):
            continue
        hits.append(
            RecognizerHit(
                category="PII.PHONE",
                start=m.start(1),
                end=m.end(1),
                confidence=0.9,
                text=raw,
                severity="MASK",
            )
        )
    return hits


# ---------------------------------------------------------------------------
# Turkish address heuristic
# ---------------------------------------------------------------------------
# We look for the structural markers ``Mah.``/``Mahallesi``, ``Sok.``/``Sokak``,
# ``Cad.``/``Caddesi``, ``No:``, ``D:``, ``Apt.``, with at least two of these
# co-occurring within ~80 chars. False-positive rate is necessarily higher
# than checksum-validated identifiers; we flag at confidence 0.6.

_ADDRESS_TOKENS = re.compile(
    r"\b(mah\.|mahallesi|sok\.|sokak|cad\.|caddesi|bulv\.|bulvarı|no\s*[:.]|d\s*[:.]|apt\.?|kat\s*[:.]|daire)\b",
    re.IGNORECASE,
)


def recognize_address(text: str) -> list[RecognizerHit]:
    hits: list[RecognizerHit] = []
    matches = list(_ADDRESS_TOKENS.finditer(text))
    if len(matches) < 2:
        return hits
    # Group consecutive matches that fall within 80 chars of each other.
    groups: list[list[re.Match[str]]] = []
    cur: list[re.Match[str]] = []
    for m in matches:
        if cur and m.start() - cur[-1].end() > 80:
            groups.append(cur)
            cur = []
        cur.append(m)
    if cur:
        groups.append(cur)
    for grp in groups:
        if len(grp) < 2:
            continue
        start = grp[0].start()
        end = grp[-1].end()
        hits.append(
            RecognizerHit(
                category="PII.ADDRESS",
                start=start,
                end=end,
                confidence=0.6,
                text=text[start:end],
                severity="MASK",
            )
        )
    return hits


TURKISH_RECOGNIZERS: list[Callable[[str], list[RecognizerHit]]] = [
    recognize_tc_kimlik,
    recognize_iban,
    recognize_gsm,
    recognize_address,
]
