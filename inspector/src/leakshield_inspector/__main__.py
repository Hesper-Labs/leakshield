"""Entry point: ``python -m leakshield_inspector``."""

from __future__ import annotations

import asyncio
import sys

from .server import serve_async


def main() -> int:
    try:
        asyncio.run(serve_async())
    except KeyboardInterrupt:
        return 0
    return 0


if __name__ == "__main__":
    sys.exit(main())
