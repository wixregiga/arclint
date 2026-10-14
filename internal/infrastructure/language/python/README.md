# Python facts

The adapter uses the embedded tree-sitter grammar and the existing module
resolver. Python and Pyright are development tools only; the binary invokes
neither. Request `resolved_calls` or `field_types` through `Producer.Facts`.
Imports alone retain the scanner and parallel worker path. Resolution builds a
repository index only when one of the new facts is requested.

Fields are class-body annotations (including dataclasses and NamedTuple
classes), annotated `self.x` assignments, and unannotated `self.x` assignments
in methods. `ClassVar` and `InitVar` imports, including aliases, are excluded.
A class emits each field once, at its first declaration; an unannotated first
declaration stays untyped even when a later assignment has an annotation.
Calls include module-level and computed calls. `EnclosingOwner` is available
without requesting resolution and is empty outside methods.

Repository imports follow aliases and `__init__.py` re-exports to declarations.
Annotated parameters, locals and fields provide static receiver types. Python's
C3 order chooses a member's declaring class; the receiver remains the annotated
class. Protocols are ordinary classes here. Structural implementations and
return-value inference are outside this adapter's resolution contract.

Annotations follow lexical bindings, including quoted forward references and
`from __future__ import annotations`. Holds includes generic containers and
their type arguments, unions and optionals. Callable signatures contribute only
the Callable declaration, and Annotated metadata and Literal values do not
contribute held types. These distinctions follow the
[typing specification](https://typing.python.org/en/latest/spec/annotations.html).
Builtins come from the same CPython generator as the embedded stdlib table;
`go generate ./internal/infrastructure/language/python` refreshes both.

Every unresolved reference has a reason. Unannotated values, Any, callable
values and fields, dynamic attribute access, kwargs forwarding, observed monkey
patches, descriptors, custom metaclasses, unknown decorators, unknown bases,
ambiguous unions, namespace packages and competing roots remain undetermined.
A dependency annotation establishes its receiver type, but its method target
stays undetermined when the declaring member cannot be observed. Module calls
through a stdlib or manifest dependency import are external, and builtins have
an empty Package. External Package is the import-facing module; for example,
`os.getcwd` may be declared in a platform-specific stdlib module.

The adapter does not execute modules or evaluate annotations. Computed type
expressions, type aliases requiring evaluation, explicit super arguments,
multiple-base super dispatch and
nested type identities that cannot be represented unambiguously remain open.
A missing member on a fully resolved repository class uses the receiver class
and member name, as required by the cross-language fact contract.

`value_object/no-identity` and `domain_event/occurrence-time` remain milestones:
the domain has no evaluators for them. Field declarations remove the adapter
limitation but do not establish either semantic rule.

For differential validation and benchmarks on real repositories, see
[the Pyright oracle](../../../../tools/pythonoracle/README.md).
