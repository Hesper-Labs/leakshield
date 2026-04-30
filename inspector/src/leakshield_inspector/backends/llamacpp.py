"""llama.cpp backend — stubbed.

The plan is to embed ``llama-cpp-python`` and call it in-process so the
inspector does not need a sidecar. This trades operational simplicity for
heavier RAM use, which is fine for self-hosted deployments.

TODO(track-llamacpp): replace this stub with a thin wrapper around
``llama_cpp.Llama(...)`` that exposes the same ``chat`` / ``health`` /
``close`` interface. See ``docs/architecture.md`` § "Backend matrix".
"""

from __future__ import annotations


class LlamaCppBackend:
    """Placeholder — raises until the backend is implemented."""

    name = "llamacpp"

    def __init__(self) -> None:
        pass

    async def health(self) -> bool:
        raise NotImplementedError(
            "llamacpp backend not yet implemented — see docs/architecture.md; "
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
        raise NotImplementedError("llamacpp backend not yet implemented")

    async def close(self) -> None:
        return None
