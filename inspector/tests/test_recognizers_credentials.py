"""Tests for the credential / secret recognizers."""

from __future__ import annotations

from leakshield_inspector.recognizers.credentials import (
    recognize_anthropic_key,
    recognize_aws_access_key,
    recognize_generic_secret,
    recognize_openai_key,
    recognize_pem,
)


def test_openai_key():
    text = "OPENAI_API_KEY=sk-proj-1234567890abcdefghijklmn"
    hits = recognize_openai_key(text)
    assert len(hits) == 1
    assert hits[0].category == "CREDENTIAL.OPENAI_KEY"


def test_openai_key_does_not_match_anthropic_prefix():
    text = "sk-ant-api03-aBcDeFgHiJkLmNoPqRsTuVwXyZ"
    assert recognize_openai_key(text) == []


def test_anthropic_key():
    text = "key=sk-ant-api03-aBcDeFgHiJkLmNoPqRsTuVwXyZ"
    hits = recognize_anthropic_key(text)
    assert len(hits) == 1
    assert hits[0].category == "CREDENTIAL.ANTHROPIC_KEY"


def test_aws_access_key():
    text = "AWS access key AKIAIOSFODNN7EXAMPLE here."
    hits = recognize_aws_access_key(text)
    assert len(hits) == 1
    assert hits[0].text == "AKIAIOSFODNN7EXAMPLE"


def test_pem_header():
    text = (
        "-----BEGIN RSA PRIVATE KEY-----\n"
        "MIIEowIBAAKCAQEAxxxxxxxxx\n"
        "-----END RSA PRIVATE KEY-----"
    )
    hits = recognize_pem(text)
    assert len(hits) == 1


def test_generic_secret_high_entropy():
    text = "config.api_key = 'aB3xY9zP1mQ7rT5vN2wK8sJ4hF6dG'"
    hits = recognize_generic_secret(text)
    assert len(hits) == 1


def test_generic_secret_skips_placeholder():
    text = "config.api_key = 'placeholder'"
    hits = recognize_generic_secret(text)
    assert hits == []


def test_generic_secret_skips_low_entropy_token():
    text = "config.api_key = 'aaaaaaaaaaaaaaaaaaaaaaaa'"
    hits = recognize_generic_secret(text)
    assert hits == []
