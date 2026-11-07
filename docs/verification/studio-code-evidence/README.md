# Verification: read-only code evidence in Studio's map

What changed: selecting an entry or a bounded context on the map can show,
on request, where the code declares it, each contract's carrier or the
missing one, and the findings tied to it, from one run of the check
`arclint check` runs. A location opens a read-only source peek below the
map.

## The gate

`web/tests/browser/evidence.test.ts` drives the built Studio against real
`arclint map serve` processes over the identity example (YAML, with its Go
code and a remote default branch), boxoffice (CML), and an empty recording.
It passes on this branch and fails on main, whose Studio has no code
evidence:

```sh
cd web
go build -o /tmp/arclint-main <main checkout>/cmd/arclint   # main's binary
ARCLINT_BIN=/tmp/arclint-main npx playwright test tests/browser/evidence.test.ts   # 9 failed
npx playwright test tests/browser/evidence.test.ts                                  # 9 passed
```

The source reads and the attribution are proved below the browser:

- `cmd/arclint/evidence_http_test.go`: the composed server over a copy of
  the example; traversal, absolute and unclean paths, manifests, files named
  or filled like secrets, a link out of the project, stale snapshots,
  another origin, and a tree digest showing nothing in the project changes.
- `internal/application/read_source_excerpt_test.go`: bounds, text-only,
  the sensitive path and content policy, and one observation per assessment.
- `internal/infrastructure/studio/source_files_test.go` and
  `source_files_unix_test.go`: links and folder links refused, a folder
  swapped for a link or a pipe during reads never followed or waited on,
  and the size limit.
- `internal/domain/conformance/evidence_attribution_test.go`: the recorded
  invariants of EntryEvidence and EvidenceAttribution.

## Screenshots and footage

Real captures of the built Studio, never mock-ups. The same steps run
against main's binary (before) and this branch's (after):

```sh
cd web
EVIDENCE_DIR=<dir>/before ARCLINT_BIN=<main binary> npx playwright test -c playwright.evidence.config.ts capture
EVIDENCE_STAGE=after EVIDENCE_DIR=<dir>/after ARCLINT_BIN=<branch binary> npx playwright test -c playwright.evidence.config.ts capture
EVIDENCE_VIDEO=1 EVIDENCE_DIR=<dir>/after ARCLINT_BIN=<branch binary> npx playwright test -c playwright.evidence.config.ts walkthrough
ffmpeg -i <dir>/after/walkthrough.webm -c:v libx264 -pix_fmt yuv420p -movflags +faststart <dir>/after/walkthrough.mp4
```

Point `EVIDENCE_DIR` outside the repository or at the git-ignored `.tmp/`.
The capture files are not committed.
