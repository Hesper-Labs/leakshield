"""Company-custom category evaluator.

Admins describe what their company considers sensitive via four mechanisms:

1. **Keyword list**       — case-insensitive substring match (optionally
                             whole-word) with position tracking.
2. **Regex**              — compiled Python regex with position tracking.
3. **Fingerprints**       — substring match that, if hit, classifies the
                             entire prompt as the category type.
4. **Hashed directories** — bulk lists of names stored as 16-byte truncated
                             SHA-256 digests. We build a Bloom filter for
                             cheap O(1) probable-match checks and verify
                             exact hits via the hash set.

A fifth mechanism (``llm_only``) is *not* evaluated here — those categories
are pushed into the judge prompt and answered by the LLM (see
``strategies/hybrid.py``).

Why a Bloom filter at all? The directory may hold ~10k names. A naive
per-token hash + set lookup is fine, but the Bloom filter lets us short-
circuit ~99% of negative cases without computing a full SHA-256, which
matters when the prompt is paragraphs long.
"""

from __future__ import annotations

import hashlib
import re
import unicodedata
from collections.abc import Iterable
from dataclasses import dataclass

from .strategies.base import CustomCategory, Decision


# ---------------------------------------------------------------------------
# Bloom filter sized for ~10,000 directory entries at 1% false-positive rate.
#
#   m = -n * ln(p) / (ln(2)^2)
#     = -10000 * ln(0.01) / (ln(2)^2)
#     ~= 95,851 bits  →  round up to 96,000 bits = 12,000 bytes.
#   k = (m / n) * ln(2)
#     ~= 6.64           →  round up to 7 hash functions.
#
# Memory: 12 KB per directory category. Quick to build, quick to query, and
# hashes are derived from the same SHA-256 we already do for storage so we
# do not introduce another hash family.
# ---------------------------------------------------------------------------

_BLOOM_BITS = 96_000
_BLOOM_HASHES = 7


@dataclass
class CategoryHit:
    """One firing of a custom category against the prompt text."""

    category: str
    severity: Decision
    start: int
    end: int
    text: str
    mechanism: str  # "keyword" | "regex" | "fingerprint" | "directory"


class _Bloom:
    """Bare-bones bit array Bloom filter.

    We avoid the optional ``bitarray`` dependency for this internal class —
    a Python ``bytearray`` is fast enough for our 96k-bit footprint and
    keeps the dependency surface minimal. (``bitarray`` is still allowed in
    pyproject for the larger directories some users may opt into.)
    """

    def __init__(self, n_bits: int = _BLOOM_BITS, n_hashes: int = _BLOOM_HASHES) -> None:
        self.n_bits = n_bits
        self.n_hashes = n_hashes
        self.bits = bytearray((n_bits + 7) // 8)

    @staticmethod
    def _digests(value: bytes) -> list[int]:
        # Derive multiple 32-bit slots from one SHA-256 — cheap and good
        # enough for our load factor.
        d = hashlib.sha256(value).digest()
        return [
            int.from_bytes(d[i : i + 4], "big", signed=False)
            for i in range(0, _BLOOM_HASHES * 4, 4)
        ]

    def add(self, value: bytes) -> None:
        for h in self._digests(value):
            idx = h % self.n_bits
            self.bits[idx >> 3] |= 1 << (idx & 7)

    def __contains__(self, value: bytes) -> bool:
        for h in self._digests(value):
            idx = h % self.n_bits
            if not (self.bits[idx >> 3] & (1 << (idx & 7))):
                return False
        return True


def _normalize_name(name: str) -> str:
    """Match the normalization used when the directory was hashed.

    NFKC, casefolded, whitespace-collapsed. The panel applies the same
    transform before SHA-256ing so the inspector matches what it stored.
    """
    s = unicodedata.normalize("NFKC", name).casefold()
    return re.sub(r"\s+", " ", s).strip()


def _hash_token(token: str) -> bytes:
    return hashlib.sha256(_normalize_name(token).encode("utf-8")).digest()[:16]


def _all_kw_positions(text: str, kw: str, *, whole_word: bool) -> Iterable[tuple[int, int]]:
    if not kw:
        return []
    needle = kw.lower()
    haystack = text.lower()
    out: list[tuple[int, int]] = []
    i = 0
    while True:
        j = haystack.find(needle, i)
        if j < 0:
            break
        end = j + len(needle)
        if whole_word:
            left_ok = j == 0 or not text[j - 1].isalnum()
            right_ok = end == len(text) or not text[end].isalnum()
            if left_ok and right_ok:
                out.append((j, end))
        else:
            out.append((j, end))
        i = end if end > j else j + 1
    return out


# Heuristic tokenization for directory matching. We grab capitalized words
# AND whitespace-separated tokens that look like names.
_WORD_TOKEN_RE = re.compile(
    r"[A-Za-zÀ-ÖØ-öø-ÿĞŞİÇÖÜğşıçöü][A-Za-zÀ-ÖØ-öø-ÿĞŞİÇÖÜğşıçöü'\-]+"
)


def _candidate_spans(text: str) -> Iterable[tuple[str, int, int]]:
    """Yield candidate (token, start, end) tuples for directory matching.

    We emit single tokens AND adjacent 2- and 3-token windows so a "first
    last" or "first middle last" name can match the directory entry.
    """
    matches = list(_WORD_TOKEN_RE.finditer(text))
    n = len(matches)
    for i, m in enumerate(matches):
        # 1-grams
        yield m.group(), m.start(), m.end()
        # 2-grams
        if i + 1 < n:
            j = matches[i + 1]
            if j.start() - m.end() <= 3:
                yield text[m.start() : j.end()], m.start(), j.end()
        # 3-grams
        if i + 2 < n:
            j = matches[i + 1]
            k = matches[i + 2]
            if (
                j.start() - m.end() <= 3
                and k.start() - j.end() <= 3
            ):
                yield text[m.start() : k.end()], m.start(), k.end()


@dataclass
class CompiledCategory:
    """Pre-compiled / pre-indexed form of a CustomCategory.

    Compiling once per request batch (rather than per inspect call) is
    fine; tenants change their categories rarely and we expect the hybrid
    strategy to cache this compiled form keyed by ``(tenant_id, policy_version)``.
    """

    category: CustomCategory
    regex: list[re.Pattern[str]]
    bloom: _Bloom | None
    hash_set: frozenset[bytes]

    @classmethod
    def compile(cls, category: CustomCategory) -> "CompiledCategory":
        regex_compiled = [re.compile(r) for r in category.regex]
        bloom: _Bloom | None = None
        hash_set: frozenset[bytes]
        if category.directory_hashes:
            bloom = _Bloom()
            for h in category.directory_hashes:
                bloom.add(h)
            hash_set = frozenset(category.directory_hashes)
        else:
            hash_set = frozenset()
        return cls(
            category=category,
            regex=regex_compiled,
            bloom=bloom,
            hash_set=hash_set,
        )


def evaluate_categories(
    categories: Iterable[CustomCategory] | Iterable[CompiledCategory],
    text: str,
    *,
    keyword_whole_word: bool = False,
) -> list[CategoryHit]:
    """Run every mechanism for every (non-llm_only) category against ``text``.

    Returns a list of hits in document order.

    ``categories`` may be either freshly defined ``CustomCategory`` instances
    (which we compile here) or pre-compiled ``CompiledCategory`` instances
    (the hybrid strategy caches these per tenant).

    ``llm_only`` categories are ignored — the strategy injects them into
    the judge prompt instead.
    """
    hits: list[CategoryHit] = []
    text_len = len(text)
    for c in categories:
        compiled = c if isinstance(c, CompiledCategory) else CompiledCategory.compile(c)
        cat = compiled.category
        if cat.llm_only:
            continue

        # 1. Fingerprints (whole-doc severity)
        for fp in cat.fingerprints:
            if fp and fp.lower() in text.lower():
                hits.append(
                    CategoryHit(
                        category=cat.name,
                        severity=cat.severity,
                        start=0,
                        end=text_len,
                        text=text,
                        mechanism="fingerprint",
                    )
                )
                break  # any fingerprint match already covers the whole doc

        # 2. Keywords
        for kw in cat.keywords:
            for s, e in _all_kw_positions(text, kw, whole_word=keyword_whole_word):
                hits.append(
                    CategoryHit(
                        category=cat.name,
                        severity=cat.severity,
                        start=s,
                        end=e,
                        text=text[s:e],
                        mechanism="keyword",
                    )
                )

        # 3. Regex
        for pat in compiled.regex:
            for m in pat.finditer(text):
                s, e = m.start(), m.end()
                if s == e:
                    continue
                hits.append(
                    CategoryHit(
                        category=cat.name,
                        severity=cat.severity,
                        start=s,
                        end=e,
                        text=text[s:e],
                        mechanism="regex",
                    )
                )

        # 4. Hashed directories
        if compiled.bloom is not None and compiled.hash_set:
            seen_spans: set[tuple[int, int]] = set()
            for token, s, e in _candidate_spans(text):
                h = _hash_token(token)
                if h in compiled.bloom and h in compiled.hash_set:
                    if (s, e) in seen_spans:
                        continue
                    seen_spans.add((s, e))
                    hits.append(
                        CategoryHit(
                            category=cat.name,
                            severity=cat.severity,
                            start=s,
                            end=e,
                            text=token,
                            mechanism="directory",
                        )
                    )

    hits.sort(key=lambda h: (h.start, h.end))
    return hits


def hash_directory_entry(name: str) -> bytes:
    """Helper used by the panel and tests to mirror the inspector's hashing.

    Returns the 16-byte truncated SHA-256 of the normalized name. Mirrors
    ``CompiledCategory`` lookup so a panel test can pre-compute the hash
    that the inspector will look for.
    """
    return _hash_token(name)


__all__ = [
    "CategoryHit",
    "CompiledCategory",
    "evaluate_categories",
    "hash_directory_entry",
]
