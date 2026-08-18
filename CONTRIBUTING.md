# Contributing to arclint

Build the binary with `make build`; it produces `./arclint`, a single static binary.

Run the full check suite with `make ci`; it checks formatting with GolangCI-Lint, runs GolangCI-Lint and `go vet`, executes `go test ./...`, and selfchecks the repository against its own `rules.yaml`. The installed hk pre-commit hook runs the same Make targets and fixes supported formatting and lint findings first.

Rule behavior is verified by the unit suites beside each layer and by the end-to-end suite in `cmd/arclint`, which drives the compiled binary over real fixture repositories; the Go adapter's toolchain suite proves classification against `go list` over pinned real repositories and runs with the normal `go test ./...` (skipped under `-short`).

Make one commit per change; do not bundle unrelated edits into a single commit.

Write commit messages whose body states the problem the change solves, not only what was edited.

Open a pull request with the template filled in; CI must pass before review.

## The agent convergence bench (retired)

The bench that measured whether real coding agents repair architecture
violations from arclint's diagnostics was retired with the legacy
engine: its scenarios initialized builtin patterns that no longer
exist. The method and result history remain in
`docs/research/agent-convergence-bench.md`; rebuild the harness against
the current engine before the next study.
