"""Mock strategy — always returns ALLOW.

This is the default strategy in ``docker compose up`` so contributors can
exercise the full gateway pipeline (auth, routing, audit, analytics) without
downloading a model. Replace with ``hybrid``, ``specialized``, or ``judge``
once you have a backend wired up.
"""

from __future__ import annotations

from typing import Any

from .base import Decision, Filter, Message, Verdict


class MockFilter(Filter):
    """Stateless filter that allows every prompt."""

    name = "mock"

    async def inspect(
        self,
        messages: list[Message],
        config: dict[str, Any],
    ) -> Verdict:
        return Verdict(
            decision=Decision.ALLOW,
            reason="mock filter — always allows; configure a real strategy in the admin panel",
            inspector_id="mock",
        )
