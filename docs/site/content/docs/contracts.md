+++
title = "Domain Contracts"
weight = 50
+++

# Domain contracts: the built-in rules

Recording a term in the domain file (the file `rules.arclint.yaml` names
under `domain.file`, else the first of `domain.arclint.yaml`,
`domain.arclint.yml` and `domain.arclint.cml` that exists, written in YAML or
[CML](/docs/cml/)) is a claim about the code, and arclint evaluates it.
A recorded aggregate must have a root, spelled with its name wherever
the repository declares it; a recorded invariant must be a method the
root calls at every mutation; a recorded value object with an invariant
must be built through one constructor. Nothing is
installed to get this: the rules are built into the binary, composed
from the DDD meta-model that defines each building block, and they
apply the moment the recorded language is non-empty.

```console
$ arclint patterns install vertical --languages go
$ arclint domain init
$ arclint domain define bounded_context ordering --definition "Where an Attendee's order is placed and kept as struck."
$ arclint domain define aggregate Order --context ordering --definition "One purchase an Attendee places." --identity OrderID
Defined aggregate Order in context ordering.
$ arclint check .
domain.arclint.yaml:8: [warning] aggregate/protects-an-invariant aggregate Order of context ordering records no invariant; a boundary drawn around nothing that must stay consistent is not yet justified
coverage: rule aggregate/commands-named-for-behavior: 1 subject(s) could not be evaluated (unsupported)
coverage: rule aggregate/invariants-enforced-by-root: 1 subject(s) could not be evaluated (unsupported): declaration matching does not evaluate the invariant's implementation
...
contracts: ordering: 0 invariant(s), 0 assertion(s); 0 structurally checked
  no named contracts to check; coverage is not established
contracts: structural checks do not verify that check bodies enforce the recorded statements
1 active finding(s) · 0 suppressed · 0 baselined · 36 rule(s) applied
```

The Pattern brought sixteen rules; recording one context with one
aggregate brought the other twenty. An empty model (`contexts: {}`)
brings none. With no Go code yet, every rule that reads declarations or
calls has nothing to read: each reports its subject as unsupported (one
coverage line per rule, shortened above), and the warning about an
aggregate with no invariant is the only finding. Once code declares
`Order`, those rules judge it.

## Recording gaps and enforcement coverage

`arclint check` reports what it checked for each recorded invariant and
assertion. Human output summarizes each context and lists gaps. JSON keeps
its diagnostic array and adds `kind: "coverage"` entries with a `contract`
object: context, owner, kind, key, statement, status, checks, and reasons for
unperformed checks. Each check retains its rule, outcome, assurance, evidence
method, and limitations.

`structurally_checked` means the required declarations or calls were found.
An empty `Assert…` method can satisfy those checks. ArcLint does not yet
verify the method body, propagated errors, or runtime execution paths against
the statement. Missing rule selection or disablement produces `unchecked`;
unavailable analysis produces `unsupported`; unresolved ownership or partial
analysis produces `undetermined`; parse failure produces `failed`. Findings
remain findings even when suppressed or baselined. A context with no named
invariants or assertions explicitly reports that no coverage is established.

## Rules that follow calls and field types

Some built-in rules judge the role a type plays: an aggregate never calls a
repository (`repository/not-called-from-aggregates`), nothing outside an
aggregate holds one of its members (`aggregate/root-only-references`), no
aggregate holds a domain service (`domain_service/not-held-by-aggregates`),
and an entity holds its identity (`entity/identity`). Names alone
cannot decide these, so arclint resolves every call to the declaration it
reaches, and every field to the declarations it holds.

arclint resolves inside the binary, for Go, TypeScript, and Python alike.
It runs no compiler (no `go`, `tsc`, or Python) and never touches the
network; it reads the repository's own source and manifests, and, for a
Go dependency, only the package clause of its vendored or module-cache
copy.

- **Go**: arclint type-checks the repository's packages from source
  with the standard library's type checker, the files `go build` would
  compile for the machine arclint runs on. A package outside the
  repository is known by its name and by the type names the repository
  uses from it, so a call into it is *external*. Test files are not
  type-checked.
- **TypeScript**: arclint follows imports, re-exports, and declared
  types through the repository's modules, with the paths and base URL
  of `tsconfig.json`.
- **Python**: arclint follows imports, class bases, and annotations
  through the repository's modules; an unannotated value has no
  declared type.

Every reference ends as one of four answers: *declared* (a declaration
in an observed file of the repository), *external* (the standard
library, a dependency, or a builtin), *local* (a type without a
file-level name, such as an interface literal), or *undetermined*.

A reference arclint cannot follow is never guessed. A function value,
a value whose type comes from outside the repository and whose method
a repository type also declares, a file the build leaves out, or an
import no manifest provides is *undetermined*, with the reason. A
rule that proves a violation reports it regardless; a rule that finds
none but met an undetermined reference reports its subject undetermined
instead of conforming, and `arclint check` prints a coverage line
naming the first references it could not follow:

```text
coverage: rule repository/not-called-from-aggregates: 1 subject(s) undetermined;
  2 reference(s) could not be resolved: internal/order/order.go:41: call clock in
  Order.Place: clock is a field of function type; the function it holds is not declared; ...
```

A call belongs to the nearest named function or method around it. An
anonymous function is part of the method that contains it, because it
usually runs while that method runs: a Go function literal, a TypeScript
arrow function, anonymous function expression, or method of an object
literal, and a Python lambda. So a repository call inside a sort
comparator or a deferred cleanup of an aggregate method counts as the
aggregate calling the repository. A *named* function nested inside a
method, a Python `def` or a TypeScript `function f()`, is its own
function with no class, and its calls are not the method's. A decorator,
a member's computed name, and a Python parameter default run when the
class or function is defined, so their calls belong to the code around
it; a TypeScript parameter default runs on every call and belongs to the
method.

A field holds its type and the element types inside pointers, collections,
and type arguments; a function type holds nothing. A generic type's own
type parameter holds nothing either: the field that instantiates it,
`Box[OrderLine]`, holds `OrderLine`. A field's own type is the one its
values are, through pointers, the repository's type aliases, and an
optional or nullable wrapper:
`entity/identity` asks for a field whose own type is the identity, so
`id *OrderID` or `id?: OrderID` counts and a list of `OrderID` values
does not. A TypeScript `static` property is no field, the way a Python
`ClassVar` is none.

Extension rules see the same facts through `ctx.facts(path)`: `calls`,
each with its `target` and `receiver` when `resolvedCallsAvailable`, and
the `holds` and `type` of every field when `fieldTypesAvailable`.

## What the file looks like

Names are keys. Each context holds its aggregates, value objects,
events, services, specifications, relations, and open questions; each
aggregate holds its identity, its invariants, and its assertions; a
domain service holds the assertions of its own operations. This is the
YAML shape. The same language can be written in CML, as
[Domain files in CML](/docs/cml/) shows.

```yaml
version: 1
project: shop
contexts:
  ordering:
    definition: Where an Attendee's order is placed and kept as struck.
    aggregates:
      Order:
        definition: One purchase an Attendee places.
        identity: OrderID
        invariants:
          lines-frozen: A placed Order's lines never change.
        assertions:
          has-lines:
            on: Place
            statement: An Order is placed with at least one line.
    value_objects:
      Price:
        definition: What one line costs, in whole cents.
        invariants:
          whole-cents: A Price is whole cents, never negative.
```

`arclint domain define <concept> <name>` writes an entry; `arclint
domain` prints the model; `arclint domain schema --write` puts the JSON
Schema for YAML domain files under `.arclint/schemas/` for editor
completion.

## What each recording demands

Every built-in rule is listed by `arclint rules` with the origin `built
in from the DDD meta-model`; `arclint rules <id>` prints one in full.
The claims below are the rules' own words, shortened.

**A bounded context's code** is located from its recorded terms: the
declarations that spell them, found anywhere in the repository. A Zone
of rules.arclint.yaml spelled with the context's name narrows the
search to that Zone's paths; it is needed only to disambiguate, when
the repository declares a recorded term more than once. A file belongs
to one context (`bounded_context/code-held-by-one-context`), and code
of one context imports another only along a recorded relation
(`bounded_context/isolated`), in the direction the relation's kind
allows (`context_relation/imports-follow-influence`).

**An aggregate's code** is where its root is declared: the package in
Go, the file in TypeScript and Python. The root is a type spelled with
the aggregate's name (`aggregate/root-declared`); the root declares one
method per recorded invariant key, `Ensure` followed by the key
(`aggregate/invariants-enforced-by-root`); it declares no `set…` method
(`aggregate/commands-named-for-behavior`, warning); no constructor or
command of the root takes or returns another aggregate's root
(`aggregate/references-by-identity`); and it records at least one
invariant (`aggregate/protects-an-invariant`, warning).

**An invariant of an aggregate** is a method named `Ensure` followed by
its key, and every constructor and every command of the root calls it
(`invariant/enforced-at-every-mutation`). **An assertion** is a method
named `Assert` followed by its key, declared on its owner (the aggregate's
root or the domain service) and called by the operation the assertion
names (`assertion/checked-by-its-operation`).
The statement is the guarantee in the domain expert's words; the code
checks it, the recording never restates it as a post-condition.

**A value object** with an invariant declares one constructor (`New`,
`New<Name>`, `Parse<Name>`, `constructor`, or `__init__`) so a value
that violates it never exists (`value_object/constructed-through-one-door`),
and no `set…` method (`value_object/no-setters`).

**Events, services, specifications, repositories, factories, entities,
and identities** are declared types spelled with the recorded name
(`ubiquitous_language/terms-declared-in-code`, `domain_event/declared`,
`domain_service/declared`, `repository/declared`, `factory/declared`);
a specification carries `SatisfiedBy`, `satisfiedBy`, or `satisfied_by`
(`specification/satisfaction-method`); an event has no setters
(`domain_event/no-setters`).

In Go the package name completes a type name: in package `order`, `type
ID` spells `OrderID`, and `type OrderID` is accepted as well.

## The code the model above asks for

```go
// Package order holds the Order aggregate of the ordering context.
package order

import "errors"

// ID identifies one Order.
type ID string

// Order is one purchase an Attendee places.
type Order struct {
	id     ID
	placed bool
	lines  []string
}

// New opens an Order with its lines.
func New(id ID, lines []string) (Order, error) {
	o := Order{id: id, lines: append([]string(nil), lines...)}
	if err := o.EnsureLinesFrozen(); err != nil {
		return Order{}, err
	}
	return o, nil
}

// Place strikes the deal; the lines freeze from here on.
func (o *Order) Place() error {
	if err := o.AssertHasLines(); err != nil {
		return err
	}
	o.placed = true
	return o.EnsureLinesFrozen()
}

// EnsureLinesFrozen enforces lines-frozen: a placed Order's lines never change.
func (o Order) EnsureLinesFrozen() error {
	if o.placed && len(o.lines) == 0 {
		return errors.New("a placed Order keeps its lines")
	}
	return nil
}

// AssertHasLines checks has-lines on Place.
func (o Order) AssertHasLines() error {
	if len(o.lines) == 0 {
		return errors.New("an Order is placed with at least one line")
	}
	return nil
}
```

```go
package order

import "errors"

// Price is what one line costs, in whole cents.
type Price struct{ cents int64 }

// NewPrice is the one door: a Price that violates whole-cents never exists.
func NewPrice(cents int64) (Price, error) {
	if cents < 0 {
		return Price{}, errors.New("a Price is never negative")
	}
	return Price{cents: cents}, nil
}
```

`order.Order` is the only declaration in the repository spelled
`Order`, so this checks clean without a Zone declared anywhere: arclint
finds the root from the recorded term alone. Drop the
`EnsureLinesFrozen` call from `Place` and the finding lands on the
command:

```console
internal/ordering/domain/order/order.go:26: [error] invariant/enforced-at-every-mutation aggregate Order: command Place does not call EnsureLinesFrozen, so invariant lines-frozen is not enforced when it completes
```

Findings anchor where the fix goes: at the domain file (whichever file
the project records in; `domain.arclint.yaml` in these examples) when the
recording has no code, at the declaration when the code falls short.
`arclint context <path>` shows which recorded contracts anchor into a
path and at which line.

## Adopting a built-in rule

A built-in rule is adopted the way a Pattern rule is: an entry under
`rules:` keyed by its id carrying no constraint. It may change the
severity, disable the rule with a reason, exclude subjects, or
suppress findings; it may not change what the rule asserts.

```yaml
rules:
  aggregate/protects-an-invariant:
    severity: error
  aggregate/references-by-identity:
    disable: "the legacy Order.Fulfill(Shipment) signature predates this rule; tracked in ISSUE-142"
```

```console
$ arclint check .
domain.arclint.yaml:8: [error] aggregate/protects-an-invariant aggregate Order of context ordering records no invariant; ...
coverage: rule aggregate/references-by-identity is disabled: the legacy Order.Fulfill(Shipment) signature predates this rule; tracked in ISSUE-142; not evaluated
```

A ruleset never spells a rule of its own under a built-in id. To adopt
findings the code cannot yet satisfy without disabling the rule,
`arclint baseline capture` records them and `check` reports them as
baselined until the code catches up.

## The meta-model behind them

The rules are composed from `metamodel.arclint.yaml` inside the binary:
one definition per building block of Domain-Driven Design, from domain
and ubiquitous language through bounded context, context relation,
entity, value object, invariant, assertion, specification, aggregate,
domain event, domain service, repository, and factory, each with cited
sources and its invariants. Each invariant that a language's file,
import, declaration, and call facts can evaluate becomes one built-in
rule under `<block>/<invariant>`; the block's invariants are one
consistency unit, so recording a block makes all of its rules apply.
`arclint agents md --write` lists the built-in rules in their own
section of `AGENTS.md` so a coding agent sees them beside the
repository's own.
