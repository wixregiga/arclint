# Contributing to arclint

Build the binary with `make build`; it produces `./arclint`, a single static binary.

Run the full check suite with `make ci`; it checks formatting with GolangCI-Lint, runs GolangCI-Lint and `go vet`, executes `go test ./...`, lints every committed JSON Schema with Spectral (`make lint-schema`, ruleset `.spectral.yaml`), and selfchecks the repository against its own `rules.arclint.yaml`. The installed hk pre-commit hook runs the same Make targets and fixes supported formatting and lint findings first.

The committed schemas are generated, never hand-edited: `docs/schemas/<name>.arclint.schema.json` are the published copies (each schema's `$id`), and `.arclint/schemas/` holds the dogfood copies the modelines of `rules.arclint.yaml` and `domain.arclint.yaml` point at. After changing a schema generator run `make schemas`, which rewrites every copy through `arclint rules schema --write` and `arclint domain schema --write`, then lints them; drift tests fail on any stale copy.

Rule behavior is verified by the unit suites beside each layer and by the end-to-end suite in `cmd/arclint`, which drives the compiled binary over real fixture repositories; the Go adapter's toolchain suite proves classification against `go list` over pinned real repositories and runs with the normal `go test ./...` (skipped under `-short`).

Make one commit per change; do not bundle unrelated edits into a single commit.

Write commit messages whose body states the problem the change solves, not only what was edited.

Open a pull request with the template filled in; CI must pass before review.

## Publishing a beta release

This checklist covers the release arclint ships today: a GitHub prerelease
containing CGO-free Linux amd64 and arm64 archives plus checksums. It does not
include package managers, stable releases, or container publishing.

### Prepare the release

- [ ] Choose the next `MAJOR.MINOR.PATCH-beta.N` version. Follow semantic
  versioning, and never reuse a version that has been published.
- [ ] Review the changes since the previous release. Write a short summary of
  what changed, who should care, and any known limitations. GoReleaser builds
  its changelog from commit subjects and omits `docs` and `chore` commits, so
  check that the remaining subjects make sense to a user.
- [ ] Confirm the README installation instructions match the archives and
  platforms this release will actually publish.
- [ ] Work from the commit intended for release, normally current `main`.
  Install the pinned tools with `mise install`, and start with a clean tree.
- [ ] Update `cmd/arclint/VERSION` and commit that version change by itself.
  This file is the version source for the CLI binary, Docker image tag,
  archives, and GitHub release.

### Prove the release locally

- [ ] Run `make ci`. This checks formatting, lint, vet, tests, the compiled
  CLI, and arclint's own architecture rules.
- [ ] Run `make leak-check`.
- [ ] Run `make release`. This validates `.goreleaser.yaml` and creates the
  Linux amd64 and arm64 archives plus `dist/checksums.txt` without publishing.
- [ ] Verify the snapshot checksums:

  ```bash
  pushd dist >/dev/null
  sha256sum -c checksums.txt
  popd >/dev/null
  ```

- [ ] Extract the archive for the current host into a temporary directory.
  Run the packaged `arclint --version`, then use that binary to run
  `arclint check .` against this repository. Remove the temporary directory.
- [ ] When Docker is available locally, build and smoke the release container.
  The release workflow always performs the version check; the second command
  below also exercises the container against this repository:

  ```bash
  version="$(cat cmd/arclint/VERSION)"
  make docker
  docker run --rm "arclint:${version}" --version
  docker run --rm -v "$PWD:/repo" "arclint:${version}" check .
  ```

### Publish

- [ ] Push the release commit and wait for its `CI` workflow to pass.
- [ ] Confirm the tree is still clean and `cmd/arclint/VERSION` still names
  the intended beta, then create and push its matching tag:

  ```bash
  version="$(cat cmd/arclint/VERSION)"
  git tag "v${version}"
  git push origin "v${version}"
  ```

  The `Release` workflow rejects any tag that is not exactly
  `vMAJOR.MINOR.PATCH-beta.N` or does not match `cmd/arclint/VERSION`. It reruns
  `make ci`, verifies the Docker image version, and publishes through
  GoReleaser.

### Verify the published release

- [ ] Wait for the `Release` workflow to pass.
- [ ] Confirm the GitHub release is marked as a prerelease, is not marked
  latest, and contains exactly `arclint_<version>_linux_amd64.tar.gz`,
  `arclint_<version>_linux_arm64.tar.gz`, and `checksums.txt`.
- [ ] Download the amd64 archive and checksums into a clean temporary
  directory. Verify the checksum, run `arclint --version`, and run
  `arclint check` against a small real repository.
- [ ] Replace raw generated notes with the prepared release summary when they
  are incomplete or unclear.
- [ ] If publishing or the smoke check fails, do not move the tag or replace
  its artifacts. Fix the problem, increment `beta.N`, and repeat the
  checklist.
