# Domain-sweep lexer oracles

The domain-sweep skill's inventory script
(`.agents/skills/domain-sweep/scripts/inventory.py`) classifies the lines of Go,
TypeScript and Python files with lexers of its own. These oracles say what each
language's own scanner implies for the same lines, by the rules the script
documents, so the script's tests can compare the two on real files.

- `main.go` uses `go/scanner`. `make check` runs it through `LexerOracleTest`
  on the repository's Go files and a fixed sample of the Go standard library;
  `make test-exhaustive` runs it on every standard library file.
- Python needs no tool here: the tests use the standard library's `tokenize`
  on the repository's Python files and the Python standard library, the same
  way.
- `typescript.cjs` uses the TypeScript parser. Node and TypeScript are used
  only by this oracle, so it runs on request. Point `ARCLINT_TYPESCRIPT` at a
  TypeScript `typescript.js` (checked with 5.8.3); the test compares the repository's
  TypeScript files and the compiler's own library files beside it:

```sh
dir=$(mktemp -d)
npm install --prefix "$dir" --ignore-scripts typescript@5.8.3
ARCLINT_TYPESCRIPT="$dir/node_modules/typescript/lib/typescript.js" \
  python3 -I -B -m unittest discover -s .agents/skills/domain-sweep/scripts -k typescript_lexer
```
