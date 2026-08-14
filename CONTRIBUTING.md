# Contributing to arclint

Build the binary with `make build`; it produces `./arclint` with the grammar subset build tags.

Run the full check suite with `make ci`; it runs `go vet` (including the oracle and bench tagged packages), `go test ./...`, and a selfcheck that lints this repository with its own `rules.yaml`.

Rule behavior is verified by the rule-test harness: `arclint rules test [case-file|dir]` materializes each case into a fresh tree, runs the ruleset, and asserts the complete violation set; with no argument it reads cases from `.arclint/tests`.

Make one commit per change; do not bundle unrelated edits into a single commit.

Write commit messages whose body states the problem the change solves, not only what was edited.

Open a pull request with the template filled in; CI must pass before review.
