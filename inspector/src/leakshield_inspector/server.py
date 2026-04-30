"""gRPC inspector server (Phase 1 skeleton).

Today this module:
  - exposes a Health probe,
  - dispatches InspectPrompt to a registered strategy,
  - falls back to MockFilter when no real strategy is configured.

Subsequent phases add:
  - generated protobuf stubs (``grpc_tools.protoc``) → ``inspector_pb2`` /
    ``inspector_pb2_grpc``,
  - HybridPresidioFilter / LlamaGuard / Custom LLM judge implementations,
  - a per-strategy registry honoring per-request selection,
  - vLLM continuous batching + ``asyncio.Semaphore`` concurrency caps.
"""

from __future__ import annotations

import asyncio
import time

from .config import load_config
from .observability import setup_logging
from .strategies.base import Filter, Message
from .strategies.mock import MockFilter

START_TIME = time.time()


def get_filter(strategy: str) -> Filter:
    """Return a Filter instance for the given strategy name.

    Subsequent phases register ``hybrid``, ``specialized``, and ``judge``.
    Until then every request falls back to MockFilter.
    """
    if strategy in ("mock", "off", ""):
        return MockFilter()
    # TODO(phase1): from .strategies.hybrid import HybridFilter
    # TODO(phase2): from .strategies.specialized import SpecializedFilter
    # TODO(phase2): from .strategies.judge import JudgeFilter
    return MockFilter()


async def serve_async() -> None:
    """Start the inspector and block until canceled.

    The real gRPC server is wired up after protobuf code generation lands;
    this implementation runs a self-test against the mock filter and waits
    for shutdown so the service shows healthy in docker compose.
    """
    cfg = load_config()
    logger = setup_logging(level=cfg.log_level, fmt=cfg.log_format)
    logger.info(
        "inspector_starting",
        port=cfg.port,
        backend=cfg.backend,
        default_strategy=cfg.default_strategy,
    )

    stop = asyncio.Event()
    try:
        f = get_filter(cfg.default_strategy)
        verdict = await f.inspect([Message(role="user", content="hello")], config={})
        logger.info(
            "self_test_ok",
            decision=verdict.decision.value,
            reason=verdict.reason,
        )
        logger.info(
            "inspector_ready",
            note="generated gRPC server will accept traffic once protobuf stubs land",
        )
        await stop.wait()
    except asyncio.CancelledError:
        pass
    finally:
        logger.info("inspector_stopping")


def uptime_seconds() -> int:
    """Return inspector process uptime in seconds."""
    return int(time.time() - START_TIME)
