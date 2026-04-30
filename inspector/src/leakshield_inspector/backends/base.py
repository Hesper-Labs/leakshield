"""Backend protocol — abstracts the LLM call away from the strategies."""

from __future__ import annotations

from typing import Protocol, runtime_checkable


class BackendError(RuntimeError):
    """Generic backend failure. Strategies should treat this as fail-closed."""


class BackendUnavailableError(BackendError):
    """Backend is reachable but unable to serve (e.g. model not pulled)."""


@runtime_checkable
class Backend(Protocol):
    """Pluggable LLM backend.

    All methods are async because the inspector serves gRPC traffic with
    ``grpc.aio`` and we do not want a blocking HTTP call to stall the event
    loop.
    """

    name: str

    async def health(self) -> bool:
        """Return True if the backend is reachable and ready to answer."""
        ...

    async def chat(
        self,
        model: str,
        system: str,
        user: str,
        *,
        json_schema: dict | None = None,
    ) -> str:
        """Send a single-turn chat to the backend and return the raw text.

        ``json_schema`` is a JSON Schema describing the expected output. The
        backend should pass this through to whatever structured-output API it
        supports (Ollama's ``format`` field, OpenAI's ``response_format``,
        etc.) and SHOULD fall back to a free-form string when the underlying
        model does not understand structured output. Strategies always
        re-validate the returned string against the same schema, so a
        slightly noisy response is acceptable.
        """
        ...

    async def close(self) -> None:
        """Release any underlying HTTP clients / sessions."""
        ...


def build_backend(kind: str, *, base_url: str) -> Backend:
    """Backend factory.

    Knows about the implemented backends (``mock``, ``ollama``) and emits a
    clear ``NotImplementedError`` for the stubs (``vllm``, ``llamacpp``,
    ``openai_compat``). The exception message points at the docs so the
    operator knows where to look.

    The factory deliberately avoids importing the real backend modules
    until they are needed; that keeps test runs from pulling ``httpx``
    transitive dependencies they do not exercise.
    """
    if kind in ("mock", "off", ""):
        from .mock import MockBackend

        return MockBackend()
    if kind == "ollama":
        from .ollama import OllamaBackend

        return OllamaBackend(base_url=base_url)
    if kind == "vllm":
        from .vllm import VLLMBackend

        return VLLMBackend(base_url=base_url)
    if kind == "llamacpp":
        from .llamacpp import LlamaCppBackend

        return LlamaCppBackend()
    if kind == "openai_compat":
        from .openai_compat import OpenAICompatBackend

        return OpenAICompatBackend(base_url=base_url)
    raise ValueError(f"unknown backend: {kind!r}")
