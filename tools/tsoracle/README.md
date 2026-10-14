# TypeScript resolution oracle

This development tool compares the Go adapter with TypeScript 5.9.3's
`program.getTypeChecker()`. Node and TypeScript are used only by this tool.
The shipped binary resolves references through its existing Go parser and
configured import graph.

Run from the ArcLint repository root, after installing the target repository's
locked dependencies with lifecycle scripts disabled:

```sh
go run ./tools/tsoracle /absolute/path/to/repository > /tmp/ts-facts.json
npm exec --yes --package=typescript@5.9.3 -- \
  node tools/tsoracle/check.cjs /tmp/ts-facts.json > /tmp/ts-report.json
```

The checker uses the nearest `tsconfig.json`, its compiler options, and observed
TypeScript files. Files selected by the configuration also participate in the
compiler program. The adapter receives the repository's source and configuration
files; dependency directories, Git internals, build outputs, and session files
are excluded. The Go driver validates the facts through `NewObservations` before
writing them.

The comparison checks every resolved call target, receiver, and held type.
Declared references must agree on file, owner, and name. Builtins must agree
with the checker's declaration or primitive type. External references use the
specifier and exported identity that the checker follows to a dependency
symbol; this preserves renamed imports and CommonJS default imports. For an
external member, the checker must reach both the imported binding and the
member. Receiver annotations are checked as written, as the facts contract
requires, even when the checker narrows an expression's type at a use site.
Alias contents and collection elements are checked separately; types inside
callable signatures do not become held values.

Calls are matched by source line, callee, and occurrence order. This accommodates
the grammars' different representations of tagged templates and automatic
semicolon insertion. A resolved reference with no matching compiler expression
fails. Undetermined references are counted by reason and do not count as
agreements. Files without available semantic facts are reported separately.
The process exits 1 for any disagreement and prints every disagreement.

To check that an apparently green result can still reject incorrect data:

```sh
npm exec --yes --package=typescript@5.9.3 -- \
  node tools/tsoracle/negative.cjs /tmp/ts-facts.json
```

The negative controls require a real repository containing declared targets,
external targets, builtin members, declared receivers, and declared field types.
They corrupt one reference of each kind and require the checker comparison to
reject each change. `white-label` supplies all five kinds.

The compiler API and module behavior are documented by TypeScript:
[compiler API](https://github.com/microsoft/TypeScript/wiki/Using-the-Compiler-API)
and [modules reference](https://www.typescriptlang.org/docs/handbook/modules/reference.html).

## Adapter coverage

The adapter follows named, renamed, default, and namespace bindings through
explicit exports and star barrels. It resolves functions with written
implementations, annotated parameter and local receivers, local `new` class
initializers, class fields and constructor parameter properties with written
annotations, class static members, and methods in known base classes or
interfaces. Receivers retain their written type when a target belongs to a base.
Field types include aliases, union members, optional values, arrays, and generic arguments.

Builtin types and members come from generated TypeScript 5.9.3 esnext and
@types/node 24.10.1 declarations. The tables initialize only when resolution
is requested. Regenerate them with:

```sh
go generate ./internal/infrastructure/language/typescript
```

Repository `.d.ts` files supply merging guards but never declaration facts or
resolved targets. Script declarations are checked for shared global names;
explicit modules keep their lexical declarations separate. The generator also
records default-library names to prevent a script declaration from silently
merging with a builtin.

Uncertainty has a reason. Inferred factory results, reassigned inferred
bindings, `any`, `unknown`, computed calls, unresolved optional chains,
conditional/mapped/indexed-access types (including builtin type computations),
generic parameters without a concrete
type, merging, ambient modules, decorated members, export conflicts and cycles,
skipped declaration files, namespace re-exports, tuples without a named receiver,
private member invocations that require lexical private-name resolution, and
unresolved imports remain undetermined. Static
imports use the adapter's existing tsconfig and manifest resolution limits;
workspace packages without a resolved entry file stay undetermined. Decorator
calls can resolve, but types whose members a decorator can change cannot.

## Production validation

Recorded results and reproducible benchmark commands are in
[results.md](results.md).
