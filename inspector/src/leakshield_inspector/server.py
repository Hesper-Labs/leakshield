"""gRPC inspector server.

Implements all three RPCs from ``inspector/v1/inspector.proto``:

- ``InspectPrompt``  — dispatches to the configured strategy.
- ``InspectStreamWindow`` — bidirectional stream; runs the same strategy on
  every WindowChunk and stops on ``is_final``.
- ``Health``         — reports the configured backend, model, and uptime.

Concurrency is bounded by an asyncio.Semaphore so a slow LLM cannot starve
the event loop. SIGTERM / SIGINT triggers a graceful shutdown that drains
in-flight RPCs.
"""

from __future__ import annotations

import asyncio
import contextlib
import json
import os
import signal
import time
import uuid
from typing import Any, AsyncIterator

import grpc

from .backends import build_backend
from .backends.base import Backend
from .cache import build_cache
from .config import InspectorConfig, load_config
from .observability import setup_logging
from .proto import inspector_pb2 as pb2
from .proto import inspector_pb2_grpc as pb2_grpc
from .strategies.base import Decision, Filter, Message, Verdict
from .strategies.hybrid import HybridFilter
from .strategies.mock import MockFilter

START_TIME = time.time()


# ----------------------------------------------------------------------------
# Decision <-> protobuf-int helpers.
# ----------------------------------------------------------------------------
_DECISION_TO_PB = {
    Decision.ALLOW: pb2.DECISION_ALLOW,
    Decision.BLOCK: pb2.DECISION_BLOCK,
    Decision.MASK: pb2.DECISION_MASK,
    Decision.ESCALATE: pb2.DECISION_ESCALATE,
}

_STRATEGY_FROM_PB = {
    pb2.STRATEGY_UNSPECIFIED: "",
    pb2.STRATEGY_MOCK: "mock",
    pb2.STRATEGY_HYBRID: "hybrid",
    pb2.STRATEGY_SPECIALIZED: "specialized",
    pb2.STRATEGY_JUDGE: "judge",
}


def _verdict_to_pb(v: Verdict) -> pb2.InspectResponse:
    resp = pb2.InspectResponse(
        decision=_DECISION_TO_PB.get(v.decision, pb2.DECISION_UNSPECIFIED),
        reason=v.reason,
        confidence=v.confidence,
        latency_ms=v.latency_ms,
        inspector_id=v.inspector_id,
    )
    for c in v.categories:
        cat_pb = resp.categories.add()
        cat_pb.name = c.name
        cat_pb.confidence = c.confidence
        for s in c.spans:
            sp = cat_pb.spans.add()
            sp.message_index = s.message_index
            sp.start = s.start
            sp.end = s.end
    if v.redacted_messages:
        for m in v.redacted_messages:
            mm = resp.redacted_messages.add()
            mm.role = m.role
            mm.content = m.content
    return resp


def _pb_messages_to_domain(pb_messages) -> list[Message]:
    return [Message(role=m.role, content=m.content) for m in pb_messages]


def _decode_config_blob(blob: bytes) -> dict[str, Any]:
    if not blob:
        return {}
    try:
        return json.loads(blob.decode("utf-8"))
    except (UnicodeDecodeError, json.JSONDecodeError):
        return {}


# ----------------------------------------------------------------------------
# Strategy registry — built once at startup, shared across requests.
# ----------------------------------------------------------------------------
class FilterRegistry:
    """Returns the right Filter for a given strategy name.

    The hybrid strategy is built eagerly with the configured backend so we
    do not pay the wiring cost on every request. Other strategies fall back
    to mock until their tracks land.
    """

    def __init__(self, cfg: InspectorConfig, backend: Backend) -> None:
        self._cfg = cfg
        self._mock = MockFilter()
        cache = build_cache("memory")  # in-memory by default; Redis is opt-in
        self._hybrid = HybridFilter(
            backend=backend,
            cache=cache,
            judge_model=cfg.judge_model,
            inspector_id=cfg.inspector_id,
        )
        self._cache = cache

    def for_strategy(self, name: str) -> Filter:
        n = (name or "").lower()
        if n in ("mock", "off", "", "unspecified"):
            return self._mock
        if n == "hybrid":
            return self._hybrid
        # specialized + judge are out of scope for this track — fall back to
        # the (safer) hybrid path so a typo in policy.config does not silently
        # disable DLP.
        return self._hybrid

    async def close(self) -> None:
        await self._cache.close()


# ----------------------------------------------------------------------------
# The servicer.
# ----------------------------------------------------------------------------
class InspectorServicerImpl(pb2_grpc.InspectorServicer):
    """gRPC servicer that dispatches to the strategy registry."""

    def __init__(
        self,
        cfg: InspectorConfig,
        backend: Backend,
        registry: FilterRegistry,
        logger,
        max_inflight: int,
    ) -> None:
        self._cfg = cfg
        self._backend = backend
        self._registry = registry
        self._logger = logger
        self._semaphore = asyncio.Semaphore(max_inflight)

    async def _run_inspect(
        self, messages: list[Message], strategy_name: str, config: dict[str, Any]
    ) -> Verdict:
        async with self._semaphore:
            f = self._registry.for_strategy(strategy_name)
            t0 = time.perf_counter()
            verdict = await f.inspect(messages, config)
            verdict.latency_ms = int((time.perf_counter() - t0) * 1000)
            if not verdict.inspector_id:
                verdict.inspector_id = self._cfg.inspector_id
            return verdict

    async def InspectPrompt(
        self,
        request: pb2.InspectRequest,
        context: grpc.aio.ServicerContext,
    ) -> pb2.InspectResponse:
        request_id = str(uuid.uuid4())
        strategy_name = _STRATEGY_FROM_PB.get(request.strategy, "")
        if not strategy_name:
            strategy_name = self._cfg.default_strategy
        config = _decode_config_blob(request.config_blob)
        config.setdefault("company_id", request.company_id)
        config.setdefault("policy_version", request.policy_version)
        messages = _pb_messages_to_domain(request.messages)

        try:
            verdict = await self._run_inspect(messages, strategy_name, config)
        except Exception as e:  # noqa: BLE001 - we want to log and convert
            self._logger.exception(
                "inspect_failed",
                request_id=request_id,
                company_id=request.company_id,
                strategy=strategy_name,
                error=repr(e),
            )
            # ``abort`` raises a control-flow exception; nothing below runs.
            await context.abort(grpc.StatusCode.INTERNAL, "inspector internal error")
            raise  # appease the type-checker

        self._logger.info(
            "inspect_done",
            request_id=request_id,
            company_id=request.company_id,
            policy_id=request.policy_id,
            policy_version=request.policy_version,
            strategy=strategy_name,
            decision=verdict.decision.value,
            latency_ms=verdict.latency_ms,
            categories=[c.name for c in verdict.categories],
        )
        return _verdict_to_pb(verdict)

    async def InspectStreamWindow(
        self,
        request_iterator: AsyncIterator[pb2.WindowChunk],
        context: grpc.aio.ServicerContext,
    ) -> AsyncIterator[pb2.WindowDecision]:
        async for chunk in request_iterator:
            messages = [Message(role="assistant", content=chunk.text)]
            # Stream chunks default to hybrid since the stream is for output
            # filtering — we want the full recognizer set, not mock.
            verdict = await self._run_inspect(
                messages,
                "hybrid",
                {"company_id": chunk.company_id, "policy_id": chunk.policy_id},
            )
            decision_int = _DECISION_TO_PB.get(verdict.decision, pb2.DECISION_UNSPECIFIED)
            redacted_text = ""
            if verdict.decision == Decision.MASK and verdict.redacted_messages:
                redacted_text = verdict.redacted_messages[0].content
            out = pb2.WindowDecision(
                request_id=chunk.request_id,
                sequence=chunk.sequence,
                decision=decision_int,
                redacted_text=redacted_text,
            )
            for c in verdict.categories:
                cat_pb = out.categories.add()
                cat_pb.name = c.name
                cat_pb.confidence = c.confidence
                for s in c.spans:
                    sp = cat_pb.spans.add()
                    sp.message_index = s.message_index
                    sp.start = s.start
                    sp.end = s.end
            self._logger.info(
                "stream_chunk",
                request_id=chunk.request_id,
                company_id=chunk.company_id,
                sequence=chunk.sequence,
                decision=verdict.decision.value,
                is_final=chunk.is_final,
            )
            yield out
            if chunk.is_final:
                break

    async def Health(
        self,
        request: pb2.HealthRequest,
        context: grpc.aio.ServicerContext,
    ) -> pb2.HealthResponse:
        backend_ok = True
        try:
            backend_ok = await self._backend.health()
        except Exception:  # noqa: BLE001
            backend_ok = False
        if backend_ok:
            status = pb2.HealthResponse_STATUS_SERVING
        else:
            # Backend down but we can still serve mock — degraded.
            status = pb2.HealthResponse_STATUS_DEGRADED
        return pb2.HealthResponse(
            status=status,
            backend=self._backend.name,
            model=self._cfg.judge_model,
            uptime_seconds=int(time.time() - START_TIME),
        )


# ----------------------------------------------------------------------------
# Server lifecycle.
# ----------------------------------------------------------------------------
async def serve_async(cfg: InspectorConfig | None = None) -> None:
    """Start the inspector and block until SIGTERM/SIGINT.

    The function is also useful in tests via ``asyncio.create_task``.
    """
    cfg = cfg or load_config()
    logger = setup_logging(level=cfg.log_level, fmt=cfg.log_format)
    logger.info(
        "inspector_starting",
        port=cfg.port,
        bind=cfg.bind,
        backend=cfg.backend,
        default_strategy=cfg.default_strategy,
        inspector_id=cfg.inspector_id,
    )

    backend = build_backend(cfg.backend, base_url=cfg.backend_url)
    registry = FilterRegistry(cfg, backend)

    max_inflight = int(os.environ.get("LEAKSHIELD_INSPECTOR_MAX_INFLIGHT", "32"))
    servicer = InspectorServicerImpl(cfg, backend, registry, logger, max_inflight)

    server = grpc.aio.server()
    pb2_grpc.add_InspectorServicer_to_server(servicer, server)
    address = f"{cfg.bind}:{cfg.port}"
    server.add_insecure_port(address)

    await server.start()
    logger.info("inspector_ready", address=address)

    stop = asyncio.Event()

    def _signal_handler() -> None:
        logger.info("inspector_signal_received")
        stop.set()

    loop = asyncio.get_running_loop()
    for sig in (signal.SIGTERM, signal.SIGINT):
        with contextlib.suppress(NotImplementedError):
            loop.add_signal_handler(sig, _signal_handler)

    try:
        await stop.wait()
    finally:
        logger.info("inspector_stopping")
        # Drain in-flight RPCs for up to 10 seconds before forcing.
        await server.stop(grace=10.0)
        await registry.close()
        await backend.close()
        logger.info("inspector_stopped")


def uptime_seconds() -> int:
    """Return inspector process uptime in seconds."""
    return int(time.time() - START_TIME)
