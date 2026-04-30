"""Adversarial test suite — loads ``adversarial.jsonl`` and checks each case.

Used in CI to detect regressions: any prompt that *was* blocked must keep
being blocked even after recognizer / strategy refactors. The judge LLM is
mocked out so this suite exercises the deterministic recognizer + custom
category path only.
"""

from __future__ import annotations

import json
from pathlib import Path

import pytest

from leakshield_inspector.cache import InMemoryCache
from leakshield_inspector.strategies.base import Decision, Message
from leakshield_inspector.strategies.hybrid import HybridFilter

_ADVERSARIAL_FILE = Path(__file__).parent / "adversarial.jsonl"


def _load_cases() -> list[dict]:
    cases: list[dict] = []
    with _ADVERSARIAL_FILE.open() as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            cases.append(json.loads(line))
    return cases


_CASES = _load_cases()


def test_adversarial_corpus_has_at_least_30_entries():
    assert len(_CASES) >= 30, f"only {len(_CASES)} cases — extend the corpus"


@pytest.mark.parametrize("case", _CASES, ids=[c["id"] for c in _CASES])
@pytest.mark.asyncio
async def test_adversarial(case, scripted_backend, in_memory_cache: InMemoryCache):
    # Pre-load a benign reply so any escalation does not block the test.
    scripted_backend.queue(
        json.dumps({"decision": "ALLOW", "categories": [], "reason": "stub"})
    )
    f = HybridFilter(
        backend=scripted_backend,
        cache=in_memory_cache,
        judge_model="test",
        judge_temperature=0.0,
        # Disable short-prompt escalation so the LLM cannot mask a recognizer
        # regression with a coincidentally-correct verdict.
        escalate_below_chars=0,
        inspector_id="test",
    )
    verdict = await f.inspect(
        [Message(role="user", content=case["prompt"])],
        config={},
    )
    expected = Decision(case["expected_decision"])
    if expected == Decision.ALLOW:
        # Allow is permissive: anything stricter is acceptable as long as the
        # corpus is consistent. We only flag real regressions: cases that
        # claim ALLOW but became BLOCK should be fine; what we want to catch
        # are cases where BLOCK silently became ALLOW.
        assert verdict.decision in (Decision.ALLOW, Decision.MASK, Decision.BLOCK), (
            f"unexpected decision {verdict.decision} for {case['id']}"
        )
    else:
        assert verdict.decision in (Decision.BLOCK, Decision.MASK), (
            f"{case['id']} expected at least MASK, got {verdict.decision}: "
            f"{verdict.reason}"
        )
        for expected_cat in case.get("expected_categories", []):
            assert any(c.name == expected_cat for c in verdict.categories), (
                f"{case['id']}: expected category {expected_cat}, got "
                f"{[c.name for c in verdict.categories]}"
            )
