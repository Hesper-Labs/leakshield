"""Verdict cache.

Caching DLP verdicts per ``(tenant, policy_version, prompt_hash)`` tuple
turns repeated requests for the same prompt into a Redis hit. We cache
ONLY when the verdict is fully attributable to deterministic recognizers
OR the LLM call ran with ``temperature=0`` — caching a non-deterministic
LLM verdict would freeze a possibly wrong answer.

Why not just always cache? Because the LLM judge can swing across runs and
admins occasionally roll out a stricter policy mid-conversation. Cache hits
must reflect the same decision the inspector would make today.

The actual Redis client is optional — when Redis is not configured (and in
the tests), :class:`InMemoryCache` provides a drop-in. The hybrid strategy
selects the implementation through :func:`build_cache`.
"""

from __future__ import annotations

import hashlib
import json
import time
from dataclasses import asdict
from typing import Protocol

from .strategies.base import Category, Decision, Message, Span, Verdict


def make_cache_key(
    *,
    tenant_id: str,
    policy_version: int,
    prompt: str,
) -> str:
    """Stable cache key.

    Format: ``v:{tenant_id}:{policy_version}:{sha256(prompt)}``. The leading
    ``v:`` namespaces the key so we can change the layout in a future
    release without colliding with old entries.
    """
    digest = hashlib.sha256(prompt.encode("utf-8")).hexdigest()
    return f"v:{tenant_id}:{policy_version}:{digest}"


def serialize_verdict(v: Verdict) -> str:
    payload = {
        "decision": v.decision.value,
        "categories": [
            {
                "name": c.name,
                "confidence": c.confidence,
                "spans": [asdict(s) for s in c.spans],
            }
            for c in v.categories
        ],
        "reason": v.reason,
        "confidence": v.confidence,
        "redacted_messages": (
            [{"role": m.role, "content": m.content} for m in v.redacted_messages]
            if v.redacted_messages
            else None
        ),
        "latency_ms": v.latency_ms,
        "inspector_id": v.inspector_id,
    }
    return json.dumps(payload)


def deserialize_verdict(blob: str) -> Verdict:
    p = json.loads(blob)
    return Verdict(
        decision=Decision(p["decision"]),
        categories=[
            Category(
                name=c["name"],
                confidence=c.get("confidence", 1.0),
                spans=[Span(**s) for s in c.get("spans", [])],
            )
            for c in p.get("categories", [])
        ],
        reason=p.get("reason", ""),
        confidence=p.get("confidence", 1.0),
        redacted_messages=(
            [Message(**m) for m in p["redacted_messages"]]
            if p.get("redacted_messages")
            else None
        ),
        latency_ms=p.get("latency_ms", 0),
        inspector_id=p.get("inspector_id", ""),
    )


class VerdictCache(Protocol):
    """Async cache protocol — Redis-shaped, but easy to fake."""

    async def get(self, key: str) -> Verdict | None: ...
    async def set(self, key: str, verdict: Verdict, *, ttl_s: int) -> None: ...
    async def close(self) -> None: ...


class InMemoryCache:
    """Process-local cache. Used in tests and as a fallback when Redis is off.

    TTL is honored explicitly so behavior matches Redis. Evictions happen
    lazily on read; the cache is small enough that we do not bother with a
    background sweeper.
    """

    def __init__(self) -> None:
        self._data: dict[str, tuple[float, str]] = {}

    async def get(self, key: str) -> Verdict | None:
        entry = self._data.get(key)
        if entry is None:
            return None
        expires_at, blob = entry
        if expires_at < time.time():
            self._data.pop(key, None)
            return None
        return deserialize_verdict(blob)

    async def set(self, key: str, verdict: Verdict, *, ttl_s: int) -> None:
        self._data[key] = (time.time() + ttl_s, serialize_verdict(verdict))

    async def close(self) -> None:
        self._data.clear()


class RedisCache:
    """Redis-backed cache. Built lazily — we do not import ``redis`` until
    the operator actually points us at one (keeps the test footprint small).
    """

    def __init__(self, *, url: str) -> None:
        try:
            import redis.asyncio as redis_async  # type: ignore[import-not-found]
        except ImportError as e:  # pragma: no cover - exercised only when Redis is wired
            raise RuntimeError(
                "redis client not installed — add `redis>=5` to inspector deps "
                "or set LEAKSHIELD_INSPECTOR_CACHE=memory"
            ) from e
        self._client = redis_async.from_url(url, encoding="utf-8", decode_responses=True)

    async def get(self, key: str) -> Verdict | None:
        blob = await self._client.get(key)
        if blob is None:
            return None
        try:
            return deserialize_verdict(blob)
        except (ValueError, KeyError):
            return None

    async def set(self, key: str, verdict: Verdict, *, ttl_s: int) -> None:
        await self._client.set(key, serialize_verdict(verdict), ex=ttl_s)

    async def close(self) -> None:
        await self._client.aclose()


def build_cache(kind: str, *, url: str | None = None) -> VerdictCache:
    """Build a cache by name. ``memory`` and ``redis`` are supported."""
    if kind in ("memory", "off", ""):
        return InMemoryCache()
    if kind == "redis":
        if not url:
            raise ValueError("redis cache requires a url")
        return RedisCache(url=url)
    raise ValueError(f"unknown cache backend: {kind!r}")


def is_cacheable(verdict: Verdict, *, escalated: bool, temperature: float) -> bool:
    """Should we put ``verdict`` into the cache?

    Rules:
      - Pure-recognizer verdicts: always cacheable.
      - Escalated to LLM at temperature 0: cacheable.
      - Escalated to LLM at temperature > 0: NOT cacheable.
    """
    if not escalated:
        return True
    return temperature == 0.0


__all__ = [
    "InMemoryCache",
    "RedisCache",
    "VerdictCache",
    "build_cache",
    "deserialize_verdict",
    "is_cacheable",
    "make_cache_key",
    "serialize_verdict",
]
