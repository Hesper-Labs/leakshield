<!--
Thanks for the pull request!

Title format: Conventional Commits, e.g.
  feat(gateway): add anthropic streaming adapter
  fix(inspector): handle empty content
  docs: clarify KEK rotation
-->

## Summary

<!-- One or two sentences: what changed and why. -->

## Linked issues

<!-- Closes #123 / Refs #456. Required for non-trivial changes. -->

## Type of change

- [ ] feat — new user-visible capability
- [ ] fix — bug fix
- [ ] perf — performance improvement
- [ ] refactor — code change with no behavior change
- [ ] docs — documentation only
- [ ] test — tests only
- [ ] chore / ci — tooling, build, or release plumbing
- [ ] BREAKING CHANGE

## Component

- [ ] gateway
- [ ] inspector
- [ ] panel
- [ ] proto
- [ ] deploy (helm / compose)
- [ ] docs
- [ ] CI / repo metadata

## Checklist

- [ ] Tests added or updated and `make test` passes locally.
- [ ] Linters pass locally (`make lint`).
- [ ] Documentation updated (README, `docs/`, or inline).
- [ ] OpenAPI spec (`docs/openapi.yaml`) updated if any HTTP route or schema changed.
- [ ] No secrets, prompts, customer data, or vendor credentials in the diff or test fixtures.
- [ ] Migration files for schema changes are forward-only and idempotent.
- [ ] Behavior under `fail_mode=closed` and `fail_mode=open` considered for any DLP-touching change.

## Risk and rollout

<!-- What could go wrong? Behind a feature flag? Migration ordering? Rollback plan? -->

## Screenshots / output

<!-- For panel changes: before/after screenshots. For gateway/inspector: relevant log excerpts. -->
