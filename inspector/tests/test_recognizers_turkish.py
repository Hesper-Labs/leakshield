"""Tests for the Turkish-aware PII recognizers."""

from __future__ import annotations

import pytest

from leakshield_inspector.recognizers.turkish import (
    _is_valid_tc_kimlik,
    recognize_address,
    recognize_gsm,
    recognize_iban,
    recognize_tc_kimlik,
)


# ----------------------------------------------------------------------------
# TC Kimlik
# ----------------------------------------------------------------------------
# 10000000146 is the example TC kimlik commonly used in Turkish public docs.
# It satisfies the official checksum.
VALID_TC = "10000000146"
# Additional constructed-valid samples used to widen coverage.
EXTRA_VALID_TC = ["12345678950", "11111111110"]

INVALID_TC = [
    "12345678901",  # bad checksum
    "00000000000",  # leading zero
    "1234567890",  # 10 digits
    "123456789012",  # 12 digits
    "abcdefghijk",
]


def test_tc_kimlik_checksum_valid():
    assert _is_valid_tc_kimlik(VALID_TC) is True


@pytest.mark.parametrize("good", EXTRA_VALID_TC)
def test_tc_kimlik_extra_valid(good):
    assert _is_valid_tc_kimlik(good) is True


@pytest.mark.parametrize("bad", INVALID_TC)
def test_tc_kimlik_invalid(bad):
    assert _is_valid_tc_kimlik(bad) is False


def test_tc_kimlik_in_text():
    text = f"My TC kimlik is {VALID_TC} please help."
    hits = recognize_tc_kimlik(text)
    assert len(hits) == 1
    assert hits[0].category == "PII.TC_KIMLIK"
    assert text[hits[0].start : hits[0].end] == VALID_TC


def test_tc_kimlik_no_false_positive_on_long_number():
    # Postal code-ish number embedded in a longer numeric blob — should not
    # fire because we anchor with non-digit boundaries.
    text = "Order number 1234567890123456 was placed."
    hits = recognize_tc_kimlik(text)
    assert hits == []


def test_tc_kimlik_invalid_checksum_in_text():
    text = "Reference: 12345678901 (looks like TC but isn't)."
    hits = recognize_tc_kimlik(text)
    assert hits == []


# ----------------------------------------------------------------------------
# IBAN
# ----------------------------------------------------------------------------
# IBAN samples (real-format checksum-validated; see ECBS examples).
VALID_IBAN_TR = "TR330006100519786457841326"  # ECBS test sample
VALID_IBAN_DE = "DE89370400440532013000"      # ECBS test sample
VALID_IBAN_GB = "GB82WEST12345698765432"      # ECBS test sample

INVALID_IBAN = [
    "TR330006100519786457841327",  # last digit changed
    "TR12345678901234567890",       # garbage
    "DE89370400440532013001",       # last digit wrong
]


@pytest.mark.parametrize("iban", [VALID_IBAN_TR, VALID_IBAN_DE, VALID_IBAN_GB])
def test_iban_valid(iban):
    text = f"Please send to {iban} thanks."
    hits = recognize_iban(text)
    assert len(hits) == 1, f"expected 1 match for {iban}"
    assert hits[0].category == "PII.IBAN"


@pytest.mark.parametrize("iban", INVALID_IBAN)
def test_iban_invalid(iban):
    text = f"Account: {iban}"
    hits = recognize_iban(text)
    assert hits == [], f"unexpected match for {iban}"


def test_iban_with_spaces():
    spaced = "TR33 0006 1005 1978 6457 8413 26"
    text = f"My IBAN: {spaced}"
    hits = recognize_iban(text)
    assert len(hits) == 1


# ----------------------------------------------------------------------------
# GSM
# ----------------------------------------------------------------------------
@pytest.mark.parametrize(
    "phone",
    [
        "+90 532 123 45 67",
        "0090 532 123 45 67",
        "0532 123 45 67",
        "0 532 123 45 67",
        "+905321234567",
        "5321234567",
    ],
)
def test_gsm_recognized(phone):
    text = f"Call me at {phone} after 5pm."
    hits = recognize_gsm(text)
    assert len(hits) >= 1, f"expected GSM match for {phone}"
    assert hits[0].category == "PII.PHONE"


@pytest.mark.parametrize(
    "not_phone",
    [
        "1234567890",  # not starting with 5
        "0212 555 11 22",  # landline (212 area code), not GSM
        "12345",  # too short
    ],
)
def test_gsm_no_false_positive(not_phone):
    text = f"Reference {not_phone}"
    hits = recognize_gsm(text)
    assert hits == [], f"unexpected GSM match for {not_phone}"


# ----------------------------------------------------------------------------
# Address
# ----------------------------------------------------------------------------
def test_address_two_markers_fires():
    text = "Atatürk Mah. Cumhuriyet Cad. No: 12 Daire 5 Kadıköy"
    hits = recognize_address(text)
    assert len(hits) == 1
    assert hits[0].category == "PII.ADDRESS"


def test_address_single_marker_does_not_fire():
    text = "Lütfen Caddesi'nde buluşalım."
    hits = recognize_address(text)
    assert hits == []


def test_address_unrelated_text_does_not_fire():
    text = "This sentence has no address markers in it."
    hits = recognize_address(text)
    assert hits == []
