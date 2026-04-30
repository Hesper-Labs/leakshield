"""Standalone health probe used by Docker HEALTHCHECK.

Returns 0 if the inspector's gRPC port is accepting connections.
"""

from __future__ import annotations

import socket
import sys

from .config import load_config


def main() -> int:
    cfg = load_config()
    host = "127.0.0.1" if cfg.bind == "0.0.0.0" else cfg.bind
    try:
        with socket.create_connection((host, cfg.port), timeout=2):
            return 0
    except OSError as e:
        print(f"healthcheck failed: {e}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
