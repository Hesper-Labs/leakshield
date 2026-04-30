"""OpenAI-compatible HTTP backend — stubbed.

Many local inference servers (LM Studio, text-generation-webui, llama-server,
LiteLLM) expose an OpenAI-shaped ``/v1/chat/completions``. This adapter will
target that shape so the operator can point LeakShield at any of them
without code changes.

TODO(track-openai-compat): post to ``{base_url}/v1/chat/completions`` with
``response_format={"type":"json_schema", "json_schema": ...}`` when the
target supports it; otherwise pass ``response_format={"type":"json_object"}``
as a fallback. See ``docs/architecture.md`` § "Backend matrix".
"""

from __future__ import annotations


class OpenAICompatBackend:
    """Placeholder — raises until the backend is implemented."""

    name = "openai_compat"

    def __init__(self, *, base_url: str) -> None:
        self._base_url = base_url

    async def health(self) -> bool:
        raise NotImplementedError(
            "openai_compat backend not yet implemented — see docs/architecture.md; "
            "until then set LEAKSHIELD_INSPECTOR_BACKEND=mock or =ollama"
        )

    async def chat(
        self,
        model: str,
        system: str,
        user: str,
        *,
        json_schema: dict | None = None,
    ) -> str:
        raise NotImplementedError("openai_compat backend not yet implemented")

    async def close(self) -> None:
        return None
