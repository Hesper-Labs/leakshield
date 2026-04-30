"""Test fixtures shared across the inspector test suite.

These fixtures keep tests offline:

  - ``mock_backend``       — returns canned ALLOW JSON.
  - ``scripted_backend``   — replays a queue of canned replies; useful when a
                              specific test wants to exercise the LLM path.
  - ``in_memory_cache``    — process-local cache, no Redis.
  - ``hybrid_filter``      — preconfigured HybridFilter using the above.

We do not depend on a running Ollama, vLLM, or Redis; CI runs the suite as
a pure-Python tree.
"""

from __future__ import annotations

import asyncio
import json
import sys
from collections import deque
from collections.abc import AsyncIterator, Iterable
from pathlib import Path

import pytest

# Ensure the inspector package is importable when tests are launched from
# the repo root (``pytest inspector/tests``).
_PKG_ROOT = Path(__file__).resolve().parent.parent / "src"
if str(_PKG_ROOT) not in sys.path:
    sys.path.insert(0, str(_PKG_ROOT))

from leakshield_inspector.cache import InMemoryCache  # noqa: E402
from leakshield_inspector.strategies.hybrid import HybridFilter  # noqa: E402


class ScriptedBackend:
    """Replays a queue of canned replies as the LLM judge.

    Each call pops the next reply (or raises ``StopAsyncIteration`` when the
    script is exhausted). Tests assert against ``calls`` to confirm the
    judge was invoked the expected number of times with the expected
    prompts.
    """

    name = "scripted"

    def __init__(self, replies: Iterable[str] | None = None) -> None:
        self._queue: deque[str] = deque(replies or [])
        self.calls: list[dict] = []

    def queue(self, reply: str) -> None:
        self._queue.append(reply)

    async def health(self) -> bool:
        return True

    async def chat(
        self,
        model: str,
        system: str,
        user: str,
        *,
        json_schema: dict | None = None,
    ) -> str:
        self.calls.append(
            {"model": model, "system": system, "user": user, "schema": json_schema}
        )
        if not self._queue:
            # Default to ALLOW so unscripted tests do not blow up.
            return json.dumps({"decision": "ALLOW", "categories": [], "reason": "default"})
        return self._queue.popleft()

    async def close(self) -> None:
        return None


@pytest.fixture
def scripted_backend() -> ScriptedBackend:
    return ScriptedBackend()


@pytest.fixture
def mock_backend():
    from leakshield_inspector.backends.mock import MockBackend

    return MockBackend()


@pytest.fixture
def in_memory_cache() -> InMemoryCache:
    return InMemoryCache()


@pytest.fixture
def hybrid_filter(scripted_backend: ScriptedBackend, in_memory_cache: InMemoryCache):
    return HybridFilter(
        backend=scripted_backend,
        cache=in_memory_cache,
        judge_model="test",
        judge_temperature=0.0,
        escalate_below_chars=0,  # disable short-prompt fast escalation by default
        inspector_id="test",
    )
