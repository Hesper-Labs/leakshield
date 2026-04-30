"""Filter strategy abstract base.

All DLP strategies implement the ``Filter`` interface. The gateway selects a
strategy based on the ``strategy`` field in the gRPC request, which mirrors
the per-tenant policy stored in the database.
"""

from __future__ import annotations

from abc import ABC, abstractmethod
from dataclasses import dataclass, field
from enum import Enum
from typing import Any


class Decision(str, Enum):
    """Outcome of inspecting a prompt."""

    ALLOW = "ALLOW"
    BLOCK = "BLOCK"
    MASK = "MASK"
    ESCALATE = "ESCALATE"


@dataclass
class Span:
    """Character range inside a single message that matched a category."""

    message_index: int
    start: int
    end: int


@dataclass
class Category:
    """A DLP category that fired for the prompt (e.g., ``PII.TC_KIMLIK``)."""

    name: str
    confidence: float = 1.0
    spans: list[Span] = field(default_factory=list)


@dataclass
class Message:
    """A normalized chat message (role + textual content)."""

    role: str
    content: str


@dataclass
class Verdict:
    """The strategy's full decision returned to the gateway."""

    decision: Decision
    categories: list[Category] = field(default_factory=list)
    reason: str = ""
    confidence: float = 1.0
    redacted_messages: list[Message] | None = None
    latency_ms: int = 0
    inspector_id: str = ""


@dataclass
class CustomCategory:
    """A company-defined DLP category.

    Custom categories let an admin describe what *this specific company* considers
    sensitive: project codenames, customer / vendor lists, internal ticket IDs,
    contract markers, M&A discussions, source files with embedded secrets, etc.

    Any combination of mechanisms is supported; a category fires if *any* mechanism
    matches. ``llm_only`` categories are not evaluated by the regex / fingerprint
    fast path and are passed to the LLM judge instead.
    """

    name: str
    description: str = ""
    severity: Decision = Decision.BLOCK
    keywords: list[str] = field(default_factory=list)
    regex: list[str] = field(default_factory=list)
    fingerprints: list[str] = field(default_factory=list)
    directory_hashes: list[bytes] = field(default_factory=list)
    llm_only: bool = False


class Filter(ABC):
    """Abstract DLP filter strategy."""

    name: str = "base"

    @abstractmethod
    async def inspect(
        self,
        messages: list[Message],
        config: dict[str, Any],
    ) -> Verdict:
        """Inspect prompt messages and return a verdict.

        ``config`` is the strategy-specific configuration JSON loaded from the
        tenant's policy. For Hybrid / Judge strategies it includes a list of
        custom categories under ``config["custom_categories"]`` (parsed into
        :class:`CustomCategory` instances by the strategy).
        """

    async def health(self) -> bool:
        """Return True if the strategy backend is reachable and warm."""
        return True
