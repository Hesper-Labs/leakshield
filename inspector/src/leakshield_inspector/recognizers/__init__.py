"""Recognizers used by the Hybrid strategy.

Three layers ship out of the box:

  - Turkish-aware PII recognizers (``turkish``): TC kimlik with checksum,
    IBAN with MOD-97, GSM phone numbers, address heuristics.
  - Credential recognizers (``credentials``): API keys and PEM blocks.
  - Presidio-like recognizers (``presidio_adapter``): universal email and
    Luhn-validated credit cards. Implemented in-house so the inspector does
    not depend on Presidio's spaCy model in unit tests; in production an
    operator can swap in the real Presidio analyzer if desired.

A recognizer is a tiny callable that takes ``text`` and returns a list of
``RecognizerHit`` instances. They are deliberately stateless and synchronous
so they can run inside the asyncio event loop without context switching;
none of them require network calls or a model.
"""

from .credentials import CREDENTIAL_RECOGNIZERS
from .presidio_adapter import PRESIDIO_LIKE_RECOGNIZERS
from .turkish import TURKISH_RECOGNIZERS, RecognizerHit

__all__ = [
    "CREDENTIAL_RECOGNIZERS",
    "PRESIDIO_LIKE_RECOGNIZERS",
    "TURKISH_RECOGNIZERS",
    "RecognizerHit",
]
