"""vLLM backend — stubbed.

vLLM speaks the OpenAI-compatible HTTP shape, so the implementation will
share most of its body with ``openai_compat`` once it lands. Pinning this
out separately keeps the ``LEAKSHIELD_INSPECTOR_BACKEND=vllm`` knob explicit
and lets us tune things like continuous-batching / token-budgeting later
without breaking the OpenAI-compat path.

TODO(track-vllm): wire through OpenAI's ``/v1/chat/completions`` with
``response_format={"type":"json_schema", "json_schema": ...}``. See
``docs/architecture.md`` § "Backend matrix".
"""

from __future__ import annotations


class VLLMBackend:
    """Placeholder — raises until the backend is implemented."""

    name = "vllm"

    def __init__(self, *, base_url: str) -> None:
        self._base_url = base_url

    async def health(self) -> bool:  # noqa: D401
        raise NotImplementedError(
            "vllm backend not yet implemented — see docs/architecture.md "
            "§ 'Backend matrix' for the planned wiring; until then "
            "set LEAKSHIELD_INSPECTOR_BACKEND=mock or =ollama"
        )

    async def chat(
        self,
        model: str,
        system: str,
        user: str,
        *,
        json_schema: dict | None = None,
    ) -> str:
        raise NotImplementedError("vllm backend not yet implemented")

    async def close(self) -> None:
        return None
