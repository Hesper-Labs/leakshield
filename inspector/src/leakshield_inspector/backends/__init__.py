"""LLM backend adapters for the inspector.

A backend is anything that can take a system + user prompt and return a
string response (which the strategies parse as JSON). Backend selection is
controlled by ``LEAKSHIELD_INSPECTOR_BACKEND``; the default is ``mock`` so
the gateway runs end-to-end without any local LLM.

LeakShield NEVER pulls models on a contributor's behalf. Switching to a
real backend (Ollama, vLLM, llama.cpp, etc.) is the operator's choice.
"""

from .base import Backend, BackendError, BackendUnavailableError, build_backend

__all__ = [
    "Backend",
    "BackendError",
    "BackendUnavailableError",
    "build_backend",
]
