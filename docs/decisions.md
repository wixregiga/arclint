# arclint decisions log

Small ambiguities resolved in favor of the proposal's shapes, recorded per
milestone. Dates are decision dates.

## M1 (2026-08-10)

1. **Module path** `github.com/wixregiga/arclint`, matching the `origin`
   remote of the repository.
2. **Graph-wide consumes clauses live in a top-level `dependencies:`
   list** (`layers`, `forbidden`, `independence`, `protected`, `acyclic`),
   because they span modules; per-module `consumes` carries the
   allow/deny, external, and stdlib policies. All report
   `contract: consumes`, `blame: consumer`.
3. **Module glob semantics**: a module glob matches files directly, and a
   glob that names a directory owns the whole subtree
   (`internal/features/*` covers `internal/features/alpha/alpha.go`),
   matching the proposal's `features: ["internal/features/*"]` shape.
   Membership is a set: overlapping module globs are legal.
4. **Internal allow-list semantics**: with `internal: []` (or any
   allow-list), an internal import must resolve to self or an allowed
   module; imports of internal code outside every declared module also
   violate. A `{deny: [...]}`-only mapping restricts nothing else.
5. **Protected is importer-centric**: a file is an allowed importer of a
   protected module when ANY of its modules is in the allow set, so
   overlapping umbrella modules do not create false violations
   (surfaced by self-hosting, fixed in the provider, kept as behavior).
6. **Classification** (also the oracle's ground-truth mapping): internal =
   under the owning module's path, or resolved through a local-directory
   `replace`, or a root `go.work` workspace member (go list equivalent:
   `.Module.Dir` inside the repo). Sibling modules in one repo without
   replace/workspace classify **external** (resolvable via require).
   Replace-to-local pointing outside the repo classifies internal with no
   tree directory. Version pinning on `replace old@v` is ignored at
   classification level.
7. **Unknown imports** (not stdlib, not internal, not covered by any
   require): repo-wide policy `scan.unknown_imports: warn|error|ignore`,
   default warn. Repos with no `go.mod` classify every non-stdlib import
   unknown.
8. **Files the go tool never considers** (path segments starting with `.`
   or `_`) are not import-analyzed; build-constrained files ARE analyzed,
   and the divergence from `go build` is asserted in the oracle as exactly
   the `IgnoredGoFiles` set. A second documented divergence class:
   testdata packages explicitly imported by tests (gin's protoexample) are
   covered by `go list` but excluded by arclint unless
   `scan.include_testdata` is set.
9. **Walker exclusions**: `.git`, `.hg`, `.svn`, `.arclint`, `vendor`,
   `node_modules` by name; `testdata` unless configured; symlinks never
   followed; `scan.exclude` adds doublestar globs.
10. **Rule ids**: explicit `id:` wins; defaults are
    `<module>.consumes.<aspect>`, `<module>.provides.<kind>[<i>]`,
    `<module>.invariants.<kind>[<i>]`, `dependencies.<kind>[<i>]`,
    `scan.unknown-imports`.
11. **Severity and exit codes**: severities `error|warn|info`, default
    error; `check` exits 1 only when an error-severity violation exists;
    2 for config/usage errors. Unknown-import warnings go to stderr, not
    the violation list.
12. **Cache** (`.arclint/cache.json`, written by `arclint load`): records
    the rules.yaml SHA-256 + arclint version; later commands skip schema
    and semantic validation on a fingerprint hit. Compiled artifacts are
    not serialized in M1 — regex/expr compilation is microseconds; the M2
    extension pipeline will extend the cache with transpile results where
    caching actually pays.
13. **`check [path]` selects the repository** (rules.yaml discovered
    upward from path, or `--rules`); it does not narrow checking to a
    subtree, because provides/graph contracts are repo-wide by nature.
14. **Correspondence sides derive over the whole tree** (the `files`
    regexes are anchored full-path matches and constrain scope
    themselves); `each` in registration rules is unanchored over the
    module's file paths and its full match (trailing `/` trimmed) anchors
    the violation. Match templates substitute regex-quoted capture
    values; value templates substitute raw values.
15. **Naming rules** apply to the file stem (base name minus final
    extension), ls-lint style; `regex:` alternatives are anchored.
16. **`runtime: [ts]`/`[py]` are schema-valid but rejected at load** until
    M3 implements those targets ("not supported yet" error, not silent).
17. **SARIF export deferred** (optional in the proposal, not in the M1 CLI
    contract); the `--format json` shape is the stable interchange.
18. **Oracle repo roster** (pinned SHAs in
    `internal/oracle/oracle_test.go`): spf13/cobra and gin-gonic/gin
    (single module), go-testfixtures/testfixtures (root go.work, 3
    members), opencontainers/runc (vendor/ + cgo), opentelemetry-go (28
    modules, sibling replaces). docker/cli was rejected because it has no
    go.mod (uses a custom `vendor.mod`), kubernetes/kubernetes because a
    multi-GB clone makes a poor recurring oracle.

### M1 gate results (measured 2026-08-10, WSL2 Ubuntu 24.04, go1.26.4)

- Gate 1: six fixture repos, tests written and failing before the engine
  existed; all green after.
- Gate 2: `make oracle` — 466 packages covered, 7,591 imports
  classification-asserted across the five repos, zero mismatches, zero
  crashes; divergences asserted: 35 build-constrained files, 1
  imported-testdata package.
- Gate 3: `make selfcheck` — 16 rules over 11 modules, clean, 45 files in
  5ms.
- Gate 4: `make bench` — cold start median 7.99ms over 11 runs (min 7.71,
  max 8.87; bound 100ms); 5,000-file synthetic repo median 79.6ms over 3
  runs (min 75.7, max 81.9; bound low single-digit seconds).
