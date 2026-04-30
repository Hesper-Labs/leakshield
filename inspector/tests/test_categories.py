"""Tests for the company-custom category evaluator."""

from __future__ import annotations

from leakshield_inspector.categories import (
    CompiledCategory,
    evaluate_categories,
    hash_directory_entry,
)
from leakshield_inspector.strategies.base import CustomCategory, Decision


def test_keyword_matches_substring_case_insensitive():
    cat = CustomCategory(
        name="PROJECT.BLUEMOON",
        severity=Decision.BLOCK,
        keywords=["Project Bluemoon"],
    )
    text = "Update on project bluemoon: design review on Friday."
    hits = evaluate_categories([cat], text)
    assert len(hits) == 1
    h = hits[0]
    assert h.category == "PROJECT.BLUEMOON"
    assert h.mechanism == "keyword"
    assert text[h.start : h.end].lower() == "project bluemoon"


def test_keyword_whole_word_mode_excludes_partial():
    cat = CustomCategory(
        name="PROJECT.PBM",
        severity=Decision.BLOCK,
        keywords=["PBM"],
    )
    text = "PBMTicketIDs are tracked elsewhere."
    # In whole-word mode, the bare PBM in PBMTicketIDs should not fire.
    hits = evaluate_categories([cat], text, keyword_whole_word=True)
    assert hits == []
    # Without whole-word mode, it does match (PBM is a substring).
    hits = evaluate_categories([cat], text, keyword_whole_word=False)
    assert len(hits) == 1


def test_keyword_multiple_positions():
    cat = CustomCategory(
        name="X",
        severity=Decision.MASK,
        keywords=["secret"],
    )
    text = "secret here, secret there, no secret everywhere"
    hits = evaluate_categories([cat], text)
    # Three positions for "secret".
    assert len(hits) == 3
    for h in hits:
        assert text[h.start : h.end] == "secret"


def test_regex_match():
    cat = CustomCategory(
        name="INTERNAL.TICKET",
        severity=Decision.MASK,
        regex=[r"ACME-\d{4,6}"],
    )
    text = "See ACME-12345 and ACME-2 (latter shouldn't match)."
    hits = evaluate_categories([cat], text)
    assert len(hits) == 1
    assert hits[0].text == "ACME-12345"
    assert hits[0].mechanism == "regex"


def test_fingerprint_classifies_whole_doc():
    cat = CustomCategory(
        name="DOC.CONFIDENTIAL",
        severity=Decision.BLOCK,
        fingerprints=["Confidential — Internal Only"],
    )
    text = "Confidential — Internal Only\n\nThe rest of this document is private."
    hits = evaluate_categories([cat], text)
    assert len(hits) == 1
    h = hits[0]
    assert h.start == 0
    assert h.end == len(text)
    assert h.mechanism == "fingerprint"


def test_fingerprint_no_match_when_marker_absent():
    cat = CustomCategory(
        name="DOC.CONFIDENTIAL",
        severity=Decision.BLOCK,
        fingerprints=["Confidential — Internal Only"],
    )
    text = "Just a normal email about lunch."
    hits = evaluate_categories([cat], text)
    assert hits == []


def test_directory_match_via_bloom_and_hash():
    # Mirror the panel: hash the directory entries and pass the digests in.
    employees = ["Ayşe Yılmaz", "Mehmet Demir", "John Smith"]
    hashes = [hash_directory_entry(n) for n in employees]
    cat = CustomCategory(
        name="EMPLOYEE.NAME",
        severity=Decision.MASK,
        directory_hashes=hashes,
    )
    compiled = [CompiledCategory.compile(cat)]

    text = "Please share the report with Mehmet Demir before noon."
    hits = evaluate_categories(compiled, text)
    # Expect at least one hit covering "Mehmet Demir".
    assert any(h.text == "Mehmet Demir" for h in hits)


def test_directory_no_match_when_name_absent():
    employees = ["Ayşe Yılmaz", "Mehmet Demir"]
    hashes = [hash_directory_entry(n) for n in employees]
    cat = CustomCategory(
        name="EMPLOYEE.NAME",
        severity=Decision.MASK,
        directory_hashes=hashes,
    )
    text = "Please share the report with Robert Brown before noon."
    hits = evaluate_categories([cat], text)
    assert all(h.mechanism != "directory" for h in hits)


def test_llm_only_categories_skipped():
    cat = CustomCategory(
        name="BUSINESS.MNA",
        severity=Decision.BLOCK,
        keywords=["merger"],
        llm_only=True,
    )
    text = "Pending merger with Acme."
    hits = evaluate_categories([cat], text)
    assert hits == [], "llm_only categories must not match in the rule layer"


def test_overlap_keyword_and_regex():
    """Both mechanisms can fire on the same span — they are not deduped."""
    cat = CustomCategory(
        name="X",
        severity=Decision.BLOCK,
        keywords=["ACME-12345"],
        regex=[r"ACME-\d{4,6}"],
    )
    text = "ACME-12345 is here."
    hits = evaluate_categories([cat], text)
    assert len(hits) == 2
    assert {h.mechanism for h in hits} == {"keyword", "regex"}


def test_directory_hash_normalization_matches_panel():
    """Sanity: the panel and the inspector must agree on the hash bytes."""
    a = hash_directory_entry("  Alice   Cooper ")
    b = hash_directory_entry("alice cooper")
    assert a == b


def test_hits_sorted_by_position():
    cat = CustomCategory(
        name="X",
        severity=Decision.BLOCK,
        keywords=["foo", "bar"],
    )
    text = "foo and then bar and then foo again"
    hits = evaluate_categories([cat], text)
    starts = [h.start for h in hits]
    assert starts == sorted(starts)
