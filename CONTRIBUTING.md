# Contributing to arclint

Build the binary with `make build`; it produces `./arclint`, a single static binary.

Run the full check suite with `make ci`; it checks formatting with GolangCI-Lint, runs GolangCI-Lint and `go vet`, executes `go test ./...`, and selfchecks the repository against its own `rules.yaml`. The installed hk pre-commit hook runs the same Make targets and fixes supported formatting and lint findings first.

Rule behavior is verified by the unit suites beside each layer and by the end-to-end suite in `cmd/arclint`, which drives the compiled binary over real fixture repositories; the Go adapter's toolchain suite proves classification against `go list` over pinned real repositories and runs with the normal `go test ./...` (skipped under `-short`).

Make one commit per change; do not bundle unrelated edits into a single commit.

Write commit messages whose body states the problem the change solves, not only what was edited.

Open a pull request with the template filled in; CI must pass before review.

## Publishing a beta release

Prerequisites: push access; local tools via `mise install` (includes
goreleaser). Work from a clean tree on the branch you intend to release
(usually main), with CI green on that commit.

1. Update `cmd/arclint/VERSION` to the beta version, then commit it.
   This file is the only release-version source used by the CLI binary,
   Docker image tag, archives, and GitHub release.

2. Create and push the matching tag:

   ```bash
   version="$(cat cmd/arclint/VERSION)"
   git tag "v${version}"
   git push origin "v${version}"
   ```

3. GitHub Actions workflow `Release` runs on that tag only. It runs
   `make ci`, then GoReleaser. Result: one GitHub prerelease (not
   latest) with:
   - `arclint_<version>_linux_amd64.tar.gz`
   - `arclint_<version>_linux_arm64.tar.gz`
   - `checksums.txt`

   The workflow rejects a tag that does not equal `v` plus the exact
   contents of `cmd/arclint/VERSION`.

Ordinary pushes and non-beta tags never publish.

Local dry-run (same archives and checksums, no credentials, no GitHub
release):

```bash
make release
```

or `mise run release`. Artifacts land under `dist/`.
