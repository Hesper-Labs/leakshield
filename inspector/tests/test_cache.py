"""Tests for the verdict cache."""

from __future__ import annotations

import pytest

from leakshield_inspector.cache import (
    InMemoryCache,
    deserialize_verdict,
    is_cacheable,
    make_cache_key,
    serialize_verdict,
)
from leakshield_inspector.strategies.base import (
    Category,
    Decision,
    Span,
    Verdict,
)


def test_cache_key_is_stable():
    a = make_cache_key(tenant_id="t", policy_version=1, prompt="hello")
    b = make_cache_key(tenant_id="t", policy_version=1, prompt="hello")
    assert a == b
    assert a.startswith("v:t:1:")


def test_cache_key_differs_on_policy_version():
    a = make_cache_key(tenant_id="t", policy_version=1, prompt="hello")
    b = make_cache_key(tenant_id="t", policy_version=2, prompt="hello")
    assert a != b


def test_serialize_round_trip():
    v = Verdict(
        decision=Decision.MASK,
        categories=[
            Category(
                name="PII.TC_KIMLIK",
                confidence=0.9,
                spans=[Span(message_index=0, start=2, end=13)],
            )
        ],
        reason="test",
        confidence=0.9,
        latency_ms=5,
        inspector_id="test",
    )
    blob = serialize_verdict(v)
    v2 = deserialize_verdict(blob)
    assert v2.decision == v.decision
    assert v2.categories[0].name == "PII.TC_KIMLIK"
    assert v2.categories[0].spans[0].start == 2


@pytest.mark.asyncio
async def test_in_memory_cache_get_set():
    cache = InMemoryCache()
    key = make_cache_key(tenant_id="t", policy_version=1, prompt="x")
    assert await cache.get(key) is None
    v = Verdict(decision=Decision.ALLOW, reason="ok")
    await cache.set(key, v, ttl_s=60)
    fetched = await cache.get(key)
    assert fetched is not None
    assert fetched.decision == Decision.ALLOW


def test_is_cacheable_rules():
    v = Verdict(decision=Decision.ALLOW)
    # Pure-recognizer verdict — always cacheable.
    assert is_cacheable(v, escalated=False, temperature=0.0) is True
    assert is_cacheable(v, escalated=False, temperature=0.7) is True
    # Escalated at T=0 — cacheable.
    assert is_cacheable(v, escalated=True, temperature=0.0) is True
    # Escalated at T>0 — NOT cacheable.
    assert is_cacheable(v, escalated=True, temperature=0.5) is False
