"""Generated gRPC stubs for ``proto/inspector/v1/inspector.proto``.

Regenerate with:

    cd inspector
    python -m grpc_tools.protoc \\
        --python_out=src/leakshield_inspector/proto \\
        --grpc_python_out=src/leakshield_inspector/proto \\
        --pyi_out=src/leakshield_inspector/proto \\
        --proto_path=../proto/inspector/v1 \\
        ../proto/inspector/v1/inspector.proto

After regeneration, change the line ``import inspector_pb2 as inspector__pb2``
in ``inspector_pb2_grpc.py`` to ``from . import inspector_pb2 as
inspector__pb2`` so the module remains importable as a package.
"""

from . import inspector_pb2, inspector_pb2_grpc

__all__ = ["inspector_pb2", "inspector_pb2_grpc"]
