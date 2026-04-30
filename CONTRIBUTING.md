# Contributing to LeakShield

LeakShield is open source and contributions are welcome. This document covers the basics.

## Development environment

```bash
git clone https://github.com/Hesper-Labs/leakshield
cd leakshield
docker compose up -d postgres redis
make dev          # starts gateway + inspector locally in parallel
```

To run individual services:

```bash
# Gateway (Go)
cd gateway && go run ./cmd/leakshield serve

# Inspector (Python)
cd inspector && pip install -e '.[dev]' && python -m leakshield_inspector

# Panel (Next.js, once scaffolded)
cd panel && npm install && npm run dev
```

If you want a real local LLM behind the inspector during development, start Ollama
separately and point the inspector at it:

```bash
# Pull whatever model you want — LeakShield does not download models for you.
ollama pull qwen2.5:3b-instruct
LEAKSHIELD_INSPECTOR_BACKEND=ollama LEAKSHIELD_INSPECTOR_BACKEND_URL=http://localhost:11434 \
  python -m leakshield_inspector
```

## Tests

```bash
make test         # all tests
make test-go      # gateway/
make test-py      # inspector/
make test-e2e     # panel/ Playwright (when scaffolded)
```

## Code style

- **Go**: `gofmt`, `goimports`, `staticcheck`, `golangci-lint`. Enforced in CI.
- **Python**: `ruff`, `mypy --strict`. Enforced in CI.
- **TypeScript**: `eslint`, `prettier`, `tsc --strict`. Enforced in CI.
- **Language**: source code, comments, commit messages, docs, error messages, and log output are
  **English only**.

## Commit conventions

- Use Conventional Commits: `feat(gateway): add openai adapter`, `fix(inspector): handle empty
  content`, `docs: clarify KEK rotation`, etc.
- Co-author trailers (`Co-Authored-By: ...`) are accepted.

## Pull request flow

1. Open an issue, or comment `/claim` on an existing one.
2. Branch as `feature/...` or `fix/...`.
3. Open a PR. CI must be green; one maintainer review is required.
4. **Adding a provider adapter**: one file plus one test file in `gateway/internal/provider/<name>/`.
5. **Adding a DLP strategy**: one file in `inspector/src/leakshield_inspector/strategies/<name>.py`
   plus tests in `inspector/tests/test_<name>.py`.
6. **Adding an inspector backend** (e.g., for a new local model server): one file in
   `inspector/src/leakshield_inspector/backends/<name>.py` plus tests.

## Security

Do not file security issues in the public tracker. See [SECURITY.md](SECURITY.md) for the
disclosure process.

## License

Contributions are released under the Apache 2.0 license. By opening a PR you agree to that.
