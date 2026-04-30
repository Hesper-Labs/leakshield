"""Smoke test for the gRPC server.

Bring the server up on a free port, call ``Health`` and ``InspectPrompt``,
and tear it down. The mock backend is used so no LLM is required.
"""

from __future__ import annotations

import asyncio
import json
import socket

import grpc
import pytest

from leakshield_inspector.config import InspectorConfig
from leakshield_inspector.proto import inspector_pb2 as pb2
from leakshield_inspector.proto import inspector_pb2_grpc as pb2_grpc
from leakshield_inspector.server import (
    FilterRegistry,
    InspectorServicerImpl,
)


def _free_port() -> int:
    with socket.socket() as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


@pytest.mark.asyncio
async def test_server_health_and_inspect_round_trip():
    port = _free_port()
    cfg = InspectorConfig(
        port=port,
        bind="127.0.0.1",
        backend="mock",
        default_strategy="mock",
        log_level="warning",
        log_format="console",
        inspector_id="test",
    )

    from leakshield_inspector.backends import build_backend
    from leakshield_inspector.observability import setup_logging

    backend = build_backend(cfg.backend, base_url=cfg.backend_url)
    registry = FilterRegistry(cfg, backend)
    logger = setup_logging(level=cfg.log_level, fmt=cfg.log_format)
    servicer = InspectorServicerImpl(cfg, backend, registry, logger, max_inflight=4)

    server = grpc.aio.server()
    pb2_grpc.add_InspectorServicer_to_server(servicer, server)
    server.add_insecure_port(f"127.0.0.1:{port}")
    await server.start()

    try:
        async with grpc.aio.insecure_channel(f"127.0.0.1:{port}") as ch:
            stub = pb2_grpc.InspectorStub(ch)

            # Health
            resp = await stub.Health(pb2.HealthRequest())
            assert resp.backend == "mock"
            assert resp.uptime_seconds >= 0

            # InspectPrompt — clean message via the mock strategy.
            req = pb2.InspectRequest(strategy=pb2.STRATEGY_MOCK)
            req.messages.add(role="user", content="hello world")
            verdict = await stub.InspectPrompt(req)
            assert verdict.decision == pb2.DECISION_ALLOW

            # Hybrid path: TC kimlik should BLOCK.
            req = pb2.InspectRequest(strategy=pb2.STRATEGY_HYBRID)
            req.messages.add(
                role="user", content="My TC kimlik 10000000146 thanks."
            )
            verdict = await stub.InspectPrompt(req)
            assert verdict.decision == pb2.DECISION_BLOCK

            # Stream Window — bidirectional stream with a single chunk.
            async def _gen():
                yield pb2.WindowChunk(
                    request_id="r1",
                    company_id="acme",
                    sequence=1,
                    text="Send to TR330006100519786457841326.",
                    is_final=True,
                )

            stream = stub.InspectStreamWindow(_gen())
            decisions = []
            async for resp in stream:
                decisions.append(resp.decision)
            assert decisions and decisions[0] in (
                pb2.DECISION_BLOCK,
                pb2.DECISION_MASK,
            )
    finally:
        await server.stop(grace=0.0)
        await registry.close()
        await backend.close()


@pytest.mark.asyncio
async def test_inspect_request_carries_custom_categories_via_config_blob():
    """Smoke: the config_blob → JSON → custom_categories pipeline works."""
    port = _free_port()
    cfg = InspectorConfig(
        port=port,
        bind="127.0.0.1",
        backend="mock",
        default_strategy="hybrid",
        log_level="warning",
        log_format="console",
        inspector_id="test",
    )

    from leakshield_inspector.backends import build_backend
    from leakshield_inspector.observability import setup_logging

    backend = build_backend(cfg.backend, base_url=cfg.backend_url)
    registry = FilterRegistry(cfg, backend)
    logger = setup_logging(level=cfg.log_level, fmt=cfg.log_format)
    servicer = InspectorServicerImpl(cfg, backend, registry, logger, max_inflight=4)
    server = grpc.aio.server()
    pb2_grpc.add_InspectorServicer_to_server(servicer, server)
    server.add_insecure_port(f"127.0.0.1:{port}")
    await server.start()

    try:
        async with grpc.aio.insecure_channel(f"127.0.0.1:{port}") as ch:
            stub = pb2_grpc.InspectorStub(ch)
            blob = json.dumps(
                {
                    "custom_categories": [
                        {
                            "name": "PROJECT.X",
                            "severity": "BLOCK",
                            "keywords": ["TopSecret"],
                        }
                    ]
                }
            ).encode()
            req = pb2.InspectRequest(
                strategy=pb2.STRATEGY_HYBRID,
                config_blob=blob,
            )
            req.messages.add(role="user", content="The TopSecret roadmap is...")
            verdict = await stub.InspectPrompt(req)
            assert verdict.decision == pb2.DECISION_BLOCK
            assert any(c.name == "PROJECT.X" for c in verdict.categories)
    finally:
        await server.stop(grace=0.0)
        await registry.close()
        await backend.close()
