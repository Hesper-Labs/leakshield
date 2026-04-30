# Contributing

LeakShield is open source under Apache 2.0 and welcomes contributions of every shape: code, docs,
issue triage, design feedback, examples, and translations. This page is the practical "how do I
get my change in" guide. The full code of conduct lives in
[CODE_OF_CONDUCT.md](https://github.com/Hesper-Labs/leakshield/blob/main/CODE_OF_CONDUCT.md).

## What needs help right now

The [Roadmap](Roadmap) page lists what's stubbed today; anything labelled `# planned` in
[API Reference](API-Reference) is a focused, scoped piece of work where context is already
written. Specific easy wins:

- New provider adapters (Cohere, Mistral, AWS Bedrock).
- New inspector backends (`vllm`, `llamacpp`, `openai_compat`).
- Translations for `panel/messages/*.json`.
- Wiki pages we haven't written yet (mostly visible from the sidebar).

If you want a starter task tagged `good first issue`, ask in
[Discussions](https://github.com/Hesper-Labs/leakshield/discussions) — the maintainers will tag
something on the spot rather than letting "good first issue" labels rot for months.

## Development setup

```bash
git clone https://github.com/Hesper-Labs/leakshield
cd leakshield
docker compose up -d postgres redis        # backing stores only
make dev                                   # gateway + inspector locally
cd panel && npm install && npm run dev     # panel on :3000
```

`make dev` starts the gateway and the Python inspector with `mock` backends, so no LLM weights are
involved. To exercise real DLP behaviour, also start Ollama and pull a model:

```bash
ollama pull qwen2.5:3b-instruct
LEAKSHIELD_INSPECTOR_BACKEND=ollama LEAKSHIELD_INSPECTOR_BACKEND_URL=http://localhost:11434 \
  python -m leakshield_inspector
```

## Tests

```bash
make test       # runs all of them
make test-go    # cd gateway && go test -race ./...
make test-py    # cd inspector && pytest
make test-e2e   # panel/ Playwright (when the test files land)
```

CI runs the full matrix on every PR — Go on Linux + macOS, Python 3.11 + 3.12, Node 20 + 22.

## Code style

- **Go**: `gofmt`, `goimports`, `staticcheck`, `golangci-lint`. CI fails on dirty.
- **Python**: `ruff check src tests`. CI fails on dirty.
- **TypeScript**: `eslint`, `prettier`, `tsc --strict`. CI fails on dirty.
- **All copy** — code, comments, commits, docs, error messages, log output — is **English only**.

## Commit conventions

[Conventional Commits](https://www.conventionalcommits.org/) — `feat(scope): …`, `fix(scope): …`,
`chore(scope): …`, `docs(scope): …`, `test(scope): …`. Scope is roughly the directory or feature
("provider", "panel", "inspector", "ci", "deploy"). The CHANGELOG is generated from these.

Co-authorship on AI-assisted commits is signalled with a trailer:

```
Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
```

Pure human commits don't need the trailer.

## PR flow

1. Open an issue first if your change is non-trivial. "I'd like to add X" + 3 bullets is enough.
   This avoids the heartbreak of a 600-line PR that gets a "we don't actually want this" reply.
2. Branch as `feature/<short-name>` or `fix/<short-name>`.
3. Open the PR. CI must be green; one maintainer review is required.
4. The PR template will ask for: linked issue, what changed, how it's tested, any risk.

### Provider adapters

To add a new provider:

1. One file in `gateway/internal/provider/<name>/<name>.go` implementing the `Provider`
   interface. Look at the existing adapters for the shape.
2. One test file `gateway/internal/provider/<name>/<name>_test.go` with the four standard tests
   (extract / inject roundtrip, forward-swaps-auth, stream-bytes-unchanged).
3. A blank-import side-effect line in `gateway/internal/server/server.go` to register the adapter.
4. (Optional) A row in `gateway/internal/migrations/0002_seed_provider_models.sql` for the model
   price catalog.
5. (Optional) Update the panel's provider tile selector.

### Inspector strategies / backends

Strategies live in `inspector/src/leakshield_inspector/strategies/`; backends in
`inspector/src/leakshield_inspector/backends/`. Each follows a `Filter` / `Backend` Protocol that
the registry imports by name. Stub files for `vllm`, `llamacpp`, `openai_compat` are already
there — replacing the `NotImplementedError` is the lowest-friction way to start.

Each new strategy must add at least one row to `inspector/tests/adversarial.jsonl` and pass the
existing test suite.

### Documentation

Wiki pages live in `docs/wiki/` and get pushed to the GitHub Wiki via `scripts/sync-wiki.sh`.
PRs that change `docs/wiki/` are reviewed alongside code changes; reviewers are expected to look
at both.

## Security

Security disclosures do **not** go in public issues. See
[SECURITY.md](https://github.com/Hesper-Labs/leakshield/blob/main/SECURITY.md) for the
disclosure process.

## License

Contributions are released under Apache 2.0. By opening a PR you agree to that.
