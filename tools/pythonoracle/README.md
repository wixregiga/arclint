# Python resolution oracle

Run from the repository root with Go, Python 3.11+ and npm/npx installed:

```sh
python3 tools/pythonoracle/compare.py /path/to/python/repository \
  --output python-oracle.json \
  --python-path /path/to/repository-venv/bin/python
```

The driver invokes the `pyoracle`-tagged Go exporter and a pinned Pyright
**1.1.407** language server. The exporter calls the production adapter and
records a source query for every Target, Receiver and Holds reference. The
driver asks `textDocument/definition` or `textDocument/typeDefinition` for each
resolved reference. It never inserts `reveal_type` calls or edits the target
repository. Pyright's LSP handlers are documented in its
[language server implementation](https://github.com/microsoft/pyright/blob/main/packages/pyright-internal/src/languageServerBase.ts).

A declared reference must match the repository file, declared name and owner.
An external reference must reach the same name and owner outside the repository.
A second query against an import-only document held in memory verifies its
Package: importing that symbol from the claimed module must reach the same
canonical endpoint. These probe documents never touch the filesystem.
External modules are compared at their canonical declaration endpoint: an import
through `os` can reach `posix` in typeshed. For implicit self, Pyright returns
the owning method as its type-definition location; that method's class is the
receiver. For zero-argument super, the receiver query is the enclosing class,
while the call target is checked at the original member expression.

Missing definitions and endpoint mismatches fail the run. Undetermined
references are counted by their reason. The JSON report preserves every
disagreement and the adjacent `.queries.json` file preserves all queries,
including undetermined ones. Install the target's dependencies in a scratch
virtualenv so Pyright can inspect external definitions; missing dependencies
are failures, not exclusions. `--extra-path` can supply an additional source
root when the target uses one.

The exporter is excluded from ordinary tests and the binary by its build tag.
The driver uses only Python's standard library. To benchmark the same cloned
repository in-process:

```sh
ARCLINT_PY_ORACLE_ROOT=/path/to/python/repository \
  go test -tags pyoracle ./internal/infrastructure/language/python \
  -run '^$' -bench BenchmarkPythonRepository -benchmem -benchtime=1s -count=5
```

`legacy_imports` invokes the previous producer path directly; `imports` invokes
the current public producer with no new facts. `syntax` requests declarations
and calls, and `resolution` requests both new fact classes. Compare imports
against legacy_imports to measure the branch overhead without conflating it
with the new field declaration work.

The corpus environment is pinned in `oracle-requirements.txt`:

```sh
python3 -m venv .venv-oracle
.venv-oracle/bin/pip install -r tools/pythonoracle/oracle-requirements.txt
```

Escaped forward-reference strings whose original offsets cannot be mapped
fail export explicitly. They are never omitted from the denominator.
