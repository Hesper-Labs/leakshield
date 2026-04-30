"""Inspector configuration loaded from LEAKSHIELD_INSPECTOR_* environment variables."""

from __future__ import annotations

from pydantic_settings import BaseSettings, SettingsConfigDict


class InspectorConfig(BaseSettings):
    """Runtime configuration for the inspector process."""

    model_config = SettingsConfigDict(
        env_prefix="LEAKSHIELD_INSPECTOR_",
        env_file=".env",
        extra="ignore",
    )

    port: int = 50051
    bind: str = "0.0.0.0"

    # Backend selects the model server. ``mock`` runs without any LLM and
    # always returns ALLOW — that lets the gateway be exercised end-to-end
    # before a model is configured.
    backend: str = "mock"  # mock | ollama | vllm | llamacpp | openai_compat
    backend_url: str = "http://localhost:11434"

    # Default DLP strategy when the per-tenant policy does not specify one.
    default_strategy: str = "mock"  # mock | hybrid | specialized | judge

    log_level: str = "info"
    log_format: str = "json"

    inspector_id: str = "inspector-default"

    # Strategy-specific defaults (overridable per tenant via policy.config).
    specialized_model: str = "llama-guard3:1b"
    judge_model: str = "qwen2.5:3b-instruct"

    otlp_endpoint: str | None = None


def load_config() -> InspectorConfig:
    return InspectorConfig()
