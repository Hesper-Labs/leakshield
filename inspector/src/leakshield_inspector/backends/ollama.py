"""Ollama backend — talks to a local Ollama server over HTTP.

We hit ``/api/chat`` with ``stream=false``. When ``json_schema`` is given we
pass it as the ``format`` field; modern Ollama (>= 0.5) interprets this as
JSON Schema-constrained sampling. For older Ollama or models that do not
understand structured output, we fall back to ``format="json"`` and rely on
Pydantic-side validation in the caller.

We never download a model. If ``/api/chat`` returns 404 (Ollama's response
when a model has not been pulled) we surface a clear error and the calling
strategy will route to the next available fallback (or fail closed).
"""

from __future__ import annotations

import json
import logging
from typing import Any

import httpx

from .base import BackendError, BackendUnavailableError

_log = logging.getLogger("leakshield_inspector.backends.ollama")


class OllamaBackend:
    """HTTP client for the Ollama ``/api/chat`` endpoint."""

    name = "ollama"

    def __init__(self, *, base_url: str, timeout_s: float = 30.0) -> None:
        self._base_url = base_url.rstrip("/")
        self._client = httpx.AsyncClient(timeout=timeout_s)

    async def health(self) -> bool:
        """Return True if Ollama responds to ``/api/tags`` (lists pulled models).

        We deliberately do NOT trigger a model pull from health.
        """
        try:
            r = await self._client.get(f"{self._base_url}/api/tags")
            return r.status_code == 200
        except httpx.HTTPError:
            return False

    async def chat(
        self,
        model: str,
        system: str,
        user: str,
        *,
        json_schema: dict | None = None,
    ) -> str:
        payload: dict[str, Any] = {
            "model": model,
            "messages": [
                {"role": "system", "content": system},
                {"role": "user", "content": user},
            ],
            "stream": False,
            "options": {"temperature": 0.0},
        }
        if json_schema is not None:
            # Ollama >= 0.5 supports JSON Schema-constrained sampling. Older
            # versions accept ``"json"`` only — we attempt the schema first
            # and fall back if Ollama complains with HTTP 400.
            payload["format"] = json_schema

        try:
            r = await self._client.post(f"{self._base_url}/api/chat", json=payload)
        except httpx.HTTPError as e:
            raise BackendUnavailableError(f"ollama not reachable: {e}") from e

        if r.status_code == 404:
            _log.warning(
                "ollama_model_missing model=%s — pull it with: ollama pull %s",
                model,
                model,
            )
            raise BackendUnavailableError(
                f"ollama model {model!r} not pulled (run: ollama pull {model})"
            )

        if r.status_code == 400 and json_schema is not None:
            # Retry with the legacy ``format=json`` switch.
            payload["format"] = "json"
            r = await self._client.post(f"{self._base_url}/api/chat", json=payload)

        if r.status_code >= 400:
            raise BackendError(f"ollama error {r.status_code}: {r.text[:200]}")

        body = r.json()
        # Ollama wraps the answer in ``message.content``.
        msg = body.get("message") or {}
        text = msg.get("content")
        if not isinstance(text, str):
            # Some older releases return a top-level ``response`` field
            # (``/api/generate`` shape). Tolerate that too.
            text = body.get("response", "")
        if not isinstance(text, str):
            raise BackendError(f"ollama response missing text: {json.dumps(body)[:200]}")
        return text

    async def close(self) -> None:
        await self._client.aclose()
