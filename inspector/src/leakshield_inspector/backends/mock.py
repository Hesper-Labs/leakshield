"""Mock backend — returns canned ALLOW JSON without any external call.

This is the default backend so ``docker compose up`` and the unit-test suite
both run without an LLM. It is *also* what the inspector falls back to in
production if a real backend is unhealthy and the operator has set the
escalation policy to ``fail-open`` (which we do not recommend, but it is
configurable).
"""

from __future__ import annotations

import json


class MockBackend:
    """Stateless backend that always claims health and returns ALLOW."""

    name = "mock"

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
        # The exact shape the Hybrid judge expects.
        return json.dumps(
            {
                "decision": "ALLOW",
                "categories": [],
                "reason": "mock",
            }
        )

    async def close(self) -> None:
        return None
