from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Decision(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    DECISION_UNSPECIFIED: _ClassVar[Decision]
    DECISION_ALLOW: _ClassVar[Decision]
    DECISION_BLOCK: _ClassVar[Decision]
    DECISION_MASK: _ClassVar[Decision]
    DECISION_ESCALATE: _ClassVar[Decision]

class Strategy(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    STRATEGY_UNSPECIFIED: _ClassVar[Strategy]
    STRATEGY_MOCK: _ClassVar[Strategy]
    STRATEGY_HYBRID: _ClassVar[Strategy]
    STRATEGY_SPECIALIZED: _ClassVar[Strategy]
    STRATEGY_JUDGE: _ClassVar[Strategy]
DECISION_UNSPECIFIED: Decision
DECISION_ALLOW: Decision
DECISION_BLOCK: Decision
DECISION_MASK: Decision
DECISION_ESCALATE: Decision
STRATEGY_UNSPECIFIED: Strategy
STRATEGY_MOCK: Strategy
STRATEGY_HYBRID: Strategy
STRATEGY_SPECIALIZED: Strategy
STRATEGY_JUDGE: Strategy

class InspectRequest(_message.Message):
    __slots__ = ("company_id", "policy_id", "policy_version", "messages", "strategy", "config_blob", "trace_metadata")
    class TraceMetadataEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    COMPANY_ID_FIELD_NUMBER: _ClassVar[int]
    POLICY_ID_FIELD_NUMBER: _ClassVar[int]
    POLICY_VERSION_FIELD_NUMBER: _ClassVar[int]
    MESSAGES_FIELD_NUMBER: _ClassVar[int]
    STRATEGY_FIELD_NUMBER: _ClassVar[int]
    CONFIG_BLOB_FIELD_NUMBER: _ClassVar[int]
    TRACE_METADATA_FIELD_NUMBER: _ClassVar[int]
    company_id: str
    policy_id: str
    policy_version: int
    messages: _containers.RepeatedCompositeFieldContainer[Message]
    strategy: Strategy
    config_blob: bytes
    trace_metadata: _containers.ScalarMap[str, str]
    def __init__(self, company_id: _Optional[str] = ..., policy_id: _Optional[str] = ..., policy_version: _Optional[int] = ..., messages: _Optional[_Iterable[_Union[Message, _Mapping]]] = ..., strategy: _Optional[_Union[Strategy, str]] = ..., config_blob: _Optional[bytes] = ..., trace_metadata: _Optional[_Mapping[str, str]] = ...) -> None: ...

class InspectResponse(_message.Message):
    __slots__ = ("decision", "categories", "reason", "confidence", "redacted_messages", "latency_ms", "inspector_id")
    DECISION_FIELD_NUMBER: _ClassVar[int]
    CATEGORIES_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    CONFIDENCE_FIELD_NUMBER: _ClassVar[int]
    REDACTED_MESSAGES_FIELD_NUMBER: _ClassVar[int]
    LATENCY_MS_FIELD_NUMBER: _ClassVar[int]
    INSPECTOR_ID_FIELD_NUMBER: _ClassVar[int]
    decision: Decision
    categories: _containers.RepeatedCompositeFieldContainer[Category]
    reason: str
    confidence: float
    redacted_messages: _containers.RepeatedCompositeFieldContainer[Message]
    latency_ms: int
    inspector_id: str
    def __init__(self, decision: _Optional[_Union[Decision, str]] = ..., categories: _Optional[_Iterable[_Union[Category, _Mapping]]] = ..., reason: _Optional[str] = ..., confidence: _Optional[float] = ..., redacted_messages: _Optional[_Iterable[_Union[Message, _Mapping]]] = ..., latency_ms: _Optional[int] = ..., inspector_id: _Optional[str] = ...) -> None: ...

class Category(_message.Message):
    __slots__ = ("name", "confidence", "spans")
    NAME_FIELD_NUMBER: _ClassVar[int]
    CONFIDENCE_FIELD_NUMBER: _ClassVar[int]
    SPANS_FIELD_NUMBER: _ClassVar[int]
    name: str
    confidence: float
    spans: _containers.RepeatedCompositeFieldContainer[Span]
    def __init__(self, name: _Optional[str] = ..., confidence: _Optional[float] = ..., spans: _Optional[_Iterable[_Union[Span, _Mapping]]] = ...) -> None: ...

class Span(_message.Message):
    __slots__ = ("message_index", "start", "end")
    MESSAGE_INDEX_FIELD_NUMBER: _ClassVar[int]
    START_FIELD_NUMBER: _ClassVar[int]
    END_FIELD_NUMBER: _ClassVar[int]
    message_index: int
    start: int
    end: int
    def __init__(self, message_index: _Optional[int] = ..., start: _Optional[int] = ..., end: _Optional[int] = ...) -> None: ...

class Message(_message.Message):
    __slots__ = ("role", "content")
    ROLE_FIELD_NUMBER: _ClassVar[int]
    CONTENT_FIELD_NUMBER: _ClassVar[int]
    role: str
    content: str
    def __init__(self, role: _Optional[str] = ..., content: _Optional[str] = ...) -> None: ...

class WindowChunk(_message.Message):
    __slots__ = ("request_id", "company_id", "policy_id", "sequence", "text", "is_final")
    REQUEST_ID_FIELD_NUMBER: _ClassVar[int]
    COMPANY_ID_FIELD_NUMBER: _ClassVar[int]
    POLICY_ID_FIELD_NUMBER: _ClassVar[int]
    SEQUENCE_FIELD_NUMBER: _ClassVar[int]
    TEXT_FIELD_NUMBER: _ClassVar[int]
    IS_FINAL_FIELD_NUMBER: _ClassVar[int]
    request_id: str
    company_id: str
    policy_id: str
    sequence: int
    text: str
    is_final: bool
    def __init__(self, request_id: _Optional[str] = ..., company_id: _Optional[str] = ..., policy_id: _Optional[str] = ..., sequence: _Optional[int] = ..., text: _Optional[str] = ..., is_final: bool = ...) -> None: ...

class WindowDecision(_message.Message):
    __slots__ = ("request_id", "sequence", "decision", "categories", "redacted_text")
    REQUEST_ID_FIELD_NUMBER: _ClassVar[int]
    SEQUENCE_FIELD_NUMBER: _ClassVar[int]
    DECISION_FIELD_NUMBER: _ClassVar[int]
    CATEGORIES_FIELD_NUMBER: _ClassVar[int]
    REDACTED_TEXT_FIELD_NUMBER: _ClassVar[int]
    request_id: str
    sequence: int
    decision: Decision
    categories: _containers.RepeatedCompositeFieldContainer[Category]
    redacted_text: str
    def __init__(self, request_id: _Optional[str] = ..., sequence: _Optional[int] = ..., decision: _Optional[_Union[Decision, str]] = ..., categories: _Optional[_Iterable[_Union[Category, _Mapping]]] = ..., redacted_text: _Optional[str] = ...) -> None: ...

class HealthRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class HealthResponse(_message.Message):
    __slots__ = ("status", "backend", "model", "uptime_seconds")
    class Status(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = ()
        STATUS_UNSPECIFIED: _ClassVar[HealthResponse.Status]
        STATUS_SERVING: _ClassVar[HealthResponse.Status]
        STATUS_DEGRADED: _ClassVar[HealthResponse.Status]
        STATUS_DOWN: _ClassVar[HealthResponse.Status]
    STATUS_UNSPECIFIED: HealthResponse.Status
    STATUS_SERVING: HealthResponse.Status
    STATUS_DEGRADED: HealthResponse.Status
    STATUS_DOWN: HealthResponse.Status
    STATUS_FIELD_NUMBER: _ClassVar[int]
    BACKEND_FIELD_NUMBER: _ClassVar[int]
    MODEL_FIELD_NUMBER: _ClassVar[int]
    UPTIME_SECONDS_FIELD_NUMBER: _ClassVar[int]
    status: HealthResponse.Status
    backend: str
    model: str
    uptime_seconds: int
    def __init__(self, status: _Optional[_Union[HealthResponse.Status, str]] = ..., backend: _Optional[str] = ..., model: _Optional[str] = ..., uptime_seconds: _Optional[int] = ...) -> None: ...
