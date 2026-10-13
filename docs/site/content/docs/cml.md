+++
title = "Domain files in CML"
description = "Record the project's domain in CML, the Context Mapper language, instead of YAML: how to switch, how each concept is written, and what arclint keeps."
weight = 4
+++

ArcLint records a project's domain language in one domain file. By default
that file is `domain.arclint.yaml`, written in YAML. It can also be written in
[CML](https://contextmapper.org/docs/language-reference/), the language of
Context Mapper, and a file named `domain.arclint.cml` is found by its name.
The extension of the file picks the format: `.yaml` or `.yml` is YAML, `.cml`
is CML.

ArcLint reads and writes CML itself, in Go, against the full Context Mapper
grammar. You do not need Java or a Context Mapper installation.

A project records its domain in exactly one file. `rules.arclint.yaml` may
name it under [`domain.file`](/docs/rules/#domain); when it names none, the
file is the first of `domain.arclint.yaml`, `domain.arclint.yml` and
`domain.arclint.cml` that exists, and `domain.arclint.yaml` when none does.
YAML comes first because YAML is the preferred format. Every command, hook,
check finding and report that names the domain file names that one.

A model at another path needs the setting:

```yaml
domain:
  file: docs/model.cml
```

A default-named file that is not the domain file is ignored. When the
ruleset names no file, arclint chooses one, and every command says on
stderr which default-named files it passed over, once for each. When both
`domain.arclint.yaml` and `domain.arclint.cml` exist and the ruleset names
none, arclint reads the YAML file and prints:

```text
arclint: domain.arclint.cml is ignored; the project's domain file is domain.arclint.yaml. Name the one to use under domain.file in rules.arclint.yaml, or remove the other.
```

The warning goes to stderr, so the output a command prints, JSON included,
and its exit code do not change. Name the file to use under `domain.file`, or
remove the other one. A file the ruleset names is the project's own choice,
so the default-named files beside it are ignored without a word; this
repository names `domain.arclint.yaml` and keeps a checked copy in
`domain.arclint.cml` that way.

## Why record in CML

- People who use Context Mapper get the same model. The bounded contexts,
  the context map, the aggregates, the value objects and the services are
  ordinary CML, so Context Mapper's tools read the file.
- There is one model, not a YAML file and a CML file to keep in step.
- Everything arclint does with the domain works the same: `arclint check`,
  `arclint context`, `arclint domain define` and the rest. Only the file
  format changes.
- ArcLint keeps what it does not model. A CML file can hold use cases,
  stakeholders, attributes and plain operations that arclint has no concept
  for. ArcLint leaves them where they are.

Stay with YAML when nobody on the project uses Context Mapper. The YAML
file has a [JSON Schema](/docs/cli/#schema-output) for editor completion. A
CML file has none, because the JSON Schema applies to YAML domain files only.

## Switch a project to CML

Run `convert` with the name of the new file:

```console
$ arclint domain convert domain.arclint.cml
Moved the domain from domain.arclint.yaml into domain.arclint.cml (CML): 2 contexts · 1 aggregate · 1 value object · 2 invariants · 2 assertions · 1 service · 1 question · 1 relation.
Read back, it records the same language.
rules.arclint.yaml names no domain file: domain.arclint.cml is found by its name.
Removed domain.arclint.yaml.
```

`convert` is all or nothing: if it fails, no file has changed. It first
works out every file it will touch, in memory:

1. What `rules.arclint.yaml` must say once the move is done. When the
   ruleset cannot be edited, for example because it no longer loads, the
   command stops here.
2. The new file, translated from the project's domain and read back. The
   two must record the same language, or the command stops and says where
   they part ways.

Then it writes, in this order:

3. The new file.
4. It settles `domain.file` in `rules.arclint.yaml`. When arclint finds the
   new file by its name, that is, when it is the first of
   `domain.arclint.yaml`, `domain.arclint.yml` and `domain.arclint.cml` that
   will exist once the previous file is gone, it removes the setting.
   Otherwise it names the new file. `convert docs/model.cml` writes the
   setting, and so does `convert domain.arclint.cml` while a stale
   `domain.arclint.yml` sits beside it, because arclint would find that
   `.yml` file first.
5. It removes the file the domain was recorded in before.

If any of those writes fails, the ones already made are put back, and the
error says nothing was changed. `export` works the same way: the copy is
worked out and read back in memory, and written only when it records the
same language. The same command moves a project back to YAML:
`arclint domain convert domain.arclint.yaml`.

`convert` deletes the previous file. Whatever that file held beyond
arclint's language goes with it. If the old file was CML with use cases in
it, those are not in the YAML file that replaces it.

To do the same in steps, and look before anything is deleted, use `export`:

1. `arclint domain export domain.arclint.cml` translates and proves the
   translation. The project's domain file and `rules.arclint.yaml` are left
   alone.
2. Delete the old file, and set `domain.file` to the new file in
   `rules.arclint.yaml` unless arclint finds it by its name. While both
   files exist and the ruleset names neither, arclint reads the first and
   says on stderr that it ignores the other.

`export` is also the way to hand Context Mapper users a copy while the
project keeps recording in YAML. Write the copy to a path that is not a
default name, such as `docs/context-map.cml`; a copy named
`domain.arclint.cml` beside the YAML file is ignored, and while the ruleset
names no file every command says so. It is one way: edits made to the CML
copy are never read back into the YAML file. Run it again later and the
copy is edited in place, so what the Context Mapper users added to it
stays.

A project that already has a CML model from Context Mapper needs no
translation. Name the file under `domain.file` and arclint reads it as it
is. [Reading a CML file you already have](#reading-a-cml-file-you-already-have)
says what arclint needs from it. A model that is already named
`domain.arclint.cml` needs no setting; one at another path is named under
`domain.file`. `arclint init --domain domain.arclint.cml` writes the setting
when it drafts a ruleset, because at that time no domain file exists yet and
arclint would choose `domain.arclint.yaml`. A new project can start in CML
right away: `arclint domain init --project shop` then writes a file that
holds only `Domain shop`, and `arclint domain define` fills it in.

Both commands take a path relative to the directory that holds
`rules.arclint.yaml`. The extension must be `.yaml`, `.yml` or `.cml`, and
the path may not be the file the project already records in; those are
usage errors (exit code 2). A file that already records a language is edited
in place, keeping everything arclint does not model, and a file that cannot
be read is never overwritten.

The domain-librarian skill describes the recorded model in every format.
The librarian records through `arclint domain define` and
`arclint domain remove`; ArcLint validates each write and writes the format.
The skill, AGENTS.md workflow, and hooks' instructions stay current after
conversion in either direction. The AGENTS.md block also snapshots rules and
the recorded domain; `arclint check` reports a stale snapshot with
`arclint agents md --write` as the refresh command.

Two guarantees hold for every file arclint writes:

- Writing a language as CML and reading it back gives the same language,
  with no change of order, text or names.
- The file parses under the Context Mapper grammar, and every name it refers
  to is declared in it. It is written to pass Context Mapper's validators
  without an error; arclint's tests hold every written file to the rules of
  those validators. Only the warnings listed in
  [Warnings in Context Mapper](#warnings-in-context-mapper) are possible.

## A worked example

This YAML domain has two contexts. `billing` holds an aggregate with an
identity, an invariant and an assertion, a value object with an invariant,
a domain service and an open question. `catalog` is its upstream, and
`billing` conforms to it.

```yaml
version: 1
project: shop
description: An online shop.
contexts:
  billing:
    definition: Where an Invoice is issued and paid.
    aggregates:
      Invoice:
        definition: One bill sent to a customer.
        identity: InvoiceID
        invariants:
          total-non-negative: An Invoice total is never negative.
        assertions:
          payment-received:
            on: Settle
            statement: A settled Invoice records its payment.
    value_objects:
      Money:
        definition: An amount in one currency.
        invariants:
          has-currency: A Money amount always names its currency.
    services:
      Ledger:
        definition: Keeps the record of every payment taken.
        assertions:
          entry-balanced:
            on: Record
            statement: Every recorded payment has one matching ledger entry.
    questions:
      refunds: Do refunds belong in billing or in a context of their own?
  catalog:
    definition: What is on sale.
relations:
  - from: catalog
    to: billing
    kind: conformist
    description: Billing takes the catalog's model of a product as it is.
```

This is the CML `arclint domain convert domain.arclint.cml` writes for it:

```cml
// The Ubiquitous Language of shop, recorded by ArcLint.

Domain shop {
  domainVisionStatement = "An online shop."
}

ContextMap {
  contains billing, catalog
  // @arclint description "Billing takes the catalog's model of a product as it is."
  catalog [U]->[D,CF] billing
}

BoundedContext billing {
  domainVisionStatement = "Where an Invoice is issued and paid."
  "Keeps the record of every payment taken."
  Service Ledger {
    void Record();
    "Every recorded payment has one matching ledger entry."
    void assertEntryBalanced() hint = "arclint.on=Record";
  }

  Aggregate Invoice {
    "One bill sent to a customer."
    Entity Invoice {
      aggregateRoot
      InvoiceID id key
      "An Invoice total is never negative."
      def void ensureTotalNonNegative();
      def void Settle();
      "A settled Invoice records its payment."
      def void assertPaymentReceived() hint = "arclint.on=Settle";
    }
  }

  Module billing {
    "An amount in one currency."
    ValueObject Money {
      "A Money amount always names its currency."
      def void ensureHasCurrency();
    }
  }
  // @arclint question "refunds" "Do refunds belong in billing or in a context of their own?"
}

BoundedContext catalog {
  domainVisionStatement = "What is on sale."
}
```

What to notice:

- The file order is the `Domain`, the `ContextMap`, then the bounded
  contexts in the order the YAML lists them. The map's one `contains` line
  names every context.
- The aggregate's definition is the doc string of its root `Entity`, the one
  marked `aggregateRoot`.
- The identity is an attribute marked `key`. Its type is the identity name.
- An invariant is an operation named `ensure` plus its key, with the
  statement as its doc string.
- An assertion is an operation named `assert` plus its key. The `hint`
  entry `arclint.on=Settle` says which operation it guards. That operation
  is declared too, right before the first assertion that names it. The
  service does the same with `Record`.
- The value object sits in a `Module` named for the context.
- The question has no CML construct, so it is a `// @arclint` comment at the
  end of the context. So is the relation's description.
- `[U]->[D,CF]` is the conformist relation. The upstream is written first.

## How each concept is written

| ArcLint | CML |
|---|---|
| project, description | `Domain <project> { domainVisionStatement = "<description>" }`. With no description, `Domain <project>` alone |
| context map | one unnamed `ContextMap`; its `contains` line lists every context in order. Written whenever the project has at least one context |
| bounded context | `BoundedContext <name>`; the definition is its `domainVisionStatement` |
| question | `// @arclint question "<key>" "<text>"`, last in the context's braces, in order |
| aggregate | `Aggregate <name>` holding the root `Entity` marked `aggregateRoot`; the definition is the root's doc string |
| identity | an attribute marked `key` on the root: `<Identity> id key` |
| entity (a member of an aggregate) | an `Entity` in the same `Aggregate`, with its identity as an attribute marked `key` when it has one |
| invariant | an operation `ensure<Key>` on the aggregate root or the value object, the statement as its doc string |
| assertion | an operation `assert<Key>` on the aggregate root or the domain service, the statement as its doc string and the hint entry `arclint.on=<operation>`; the operation is declared next to it |
| value object | a `ValueObject` in the context's `Module` |
| specification | a `ValueObject` in the `Module` that declares `def boolean satisfiedBy();` |
| domain event | a `DomainEvent` in the `Module`, with `{ belongsTo @<root> }` when an aggregate raises it |
| domain service | a `Service` in the `BoundedContext` |
| repository | `Repository <Name>` last in the root, when the name is an identifier ending in `Repository`; otherwise the hint entry `arclint.repository=<Name>` on the root |
| factory | a `Service` inside the `Aggregate` carrying the hint entry `arclint.role=factory` |
| alias | one hint entry `arclint.alias=<alias>` per alias, on the root, the entity or the value object |

A context's `Module` is written only when the context has a value object, a
specification or an event. Value objects come first in it, then
specifications, then events. Inside a context the order is services,
aggregates, the `Module`, then the question comments. Services come first
because Context Mapper does not parse a `Service` with a doc string that
follows a `Module` or an `Aggregate` in the same context; when you add a
documented service to a file, arclint moves the context's services ahead of
its aggregates and module for the same reason.

Context Mapper wants every `Aggregate` name to be unique across the whole
model. When two contexts record an aggregate of the same name, the
`Aggregate` wrappers are named `<context>_<name>`. The root `Entity` keeps
the aggregate's own name, and arclint never reads the wrapper's name.

An identity is always written as an attribute, never as a reference.
Context Mapper rejects a reference from an aggregate to a value object that
sits outside it.

### Relations

Each relation is one line in the `ContextMap`, written from its `From`
context to its `To` context with a role list on each side of an arrow. For
the one-way kinds `From` is the upstream context.

| kind | CML |
|---|---|
| `partnership` | `From [P]<->[P] To` |
| `shared_kernel` | `From [SK]<->[SK] To` |
| `customer_supplier` | `From [U,S]->[D,C] To` |
| `conformist` | `From [U]->[D,CF] To` |
| `anticorruption_layer` | `From [U]->[D,ACL] To` |
| `open_host_service` | `From [U,OHS]->[D] To` |
| `published_language` | `From [U,PL]->[D] To` |
| `separate_ways` | no CML relationship exists; a `// @arclint separate-ways "<from>" "<to>"` comment in the map body, at the relation's position |

A relation's description is a `// @arclint description "<text>"` comment
directly above its relationship line. A separate-ways relation carries its
description as a third argument.

## Notes ArcLint keeps outside the CML grammar

CML has no construct for a few things arclint records. ArcLint keeps them in
two places, hint entries and directives, and uses them only where CML has
no construct.

### Hint entries

Context Mapper has a `hint = "..."` slot on entities, value objects,
services and operations. ArcLint writes comma-separated `key=value` entries
into it. An aggregate root with two aliases looks like this:

```
"One bill sent to a customer." Entity Invoice {
    aggregateRoot
    hint = "arclint.alias=Bill,arclint.alias=Receipt"
    InvoiceID id key
}
```

ArcLint owns the entries whose key starts with `arclint.`:

| entry | meaning |
|---|---|
| `arclint.alias=<name>` | an alias; one entry per alias, in order |
| `arclint.on=<operation>` | the operation an assertion guards |
| `arclint.role=factory` | the service in an aggregate is its factory |
| `arclint.repository=<name>` | the aggregate's repository, when its name cannot be a CML `Repository` |

Values percent-encode three characters and nothing else: `%` as `%25`, `,`
as `%2C` and `=` as `%3D`. An alias `Bill, Receipt` is written
`arclint.alias=Bill%2C Receipt`. Entries with any other key are not
arclint's. They are kept as they are, in place, when arclint edits the file.

### Directives

A directive is a single-line comment:

```
// @arclint <kind> "<argument>" ...
```

The arguments are CML double-quoted strings, with the same escapes CML
strings take. Only a `//` comment whose text starts with `@arclint` (after
optional spaces) is a directive. Every other comment is ordinary and arclint
never interprets it.

| directive | where | meaning |
|---|---|---|
| `name "<exact>"` | directly above the declaration it names | the exact name, when the name is not a CML identifier |
| `question "<key>" "<text>"` | inside a bounded context's braces | an open question |
| `description "<text>"` | directly above a relationship | the relation's description |
| `separate-ways "<from>" "<to>"` and an optional `"<description>"` | in the context map's body | a separate-ways relation, at that position |

A directive with an unknown kind, the wrong number of arguments or a
malformed argument is an error. The error gives its position.

### Names that are not CML identifiers

A CML identifier is letters, digits and underscores, not starting with a
digit, and it may be written with a leading `^` to escape a CML keyword. An
arclint name that is an identifier is written as it is, with the `^` when it
is a keyword.

Any other name gets a handle. Context names may hold hyphens, and term names
and the project name are free text, so this happens often. To make the
handle:

1. Replace each run of characters an identifier cannot hold with `_`.
2. Put `_` in front if it would start with a digit.
3. If another name or handle in the file is already the same, add `_2`, then
   `_3`, and so on.

The declaration carries the exact name in a `name` directive. Every
reference to it uses the handle:

```
ContextMap {
    contains order_intake, billing
    order_intake [U]->[D,CF] billing
}

// @arclint name "order-intake"
BoundedContext order_intake {
    domainVisionStatement = "Where an order is taken."
}
```

The `billing` context follows `order_intake` in the file; it is left out
here. When it reads the file, arclint resolves the handle through the
directive.

An identity that is not a value object arclint records and whose name is not
an identifier carries the `name` directive on the identity attribute.

## Invariant and assertion keys

Invariants and assertions have keys in lower-case kebab-case:
`total-non-negative`. CML operation names cannot hold hyphens, so the key
becomes a name in camel case, after `ensure` for an invariant and after
`assert` for an assertion: the invariant `total-non-negative` is the
operation `ensureTotalNonNegative`, and the assertion `payment-received` is
`assertPaymentReceived`.

The rule, called KeyCase:

- Split the key at its hyphens.
- A part that starts with a letter is written with that letter in upper
  case.
- A part that starts with a digit is written as `_` followed by the part.

| key | invariant operation |
|---|---|
| `published-only` | `ensurePublishedOnly` |
| `tier-2-pricing` | `ensureTier_2Pricing` |
| `x-1` | `ensureX_1` |
| `x1` | `ensureX1` |
| `a-b2-c3d` | `ensureAB2C3d` |

The `_` is what keeps `x-1` and `x1` apart. Reading reverses it: a new part
starts at every upper-case letter, which is lower-cased, and at every `_`,
which is dropped. Every key comes back as it went in.

## Reading a CML file you already have

ArcLint reads a `.cml` domain file written by someone else, such as a Context
Mapper model. It takes the parts that are arclint's language and leaves the
rest alone.

If the file cannot become a valid domain, arclint lists every problem it
found, each with file, line and column, and then fails. It never uses part
of a file.

### What it needs

- A definition for every term, as a doc string. A bounded context may use
  its `domainVisionStatement` instead. For a bounded context, an aggregate
  root or a value object, a comment written directly above the declaration
  serves when there is no doc string or vision statement.
- One `Entity` marked `aggregateRoot` in each `Aggregate`. Two roots in one
  `Aggregate` are an error, and Context Mapper rejects them too. An
  `Aggregate` with no root is not an arclint aggregate; its value objects,
  events and services are still read.
- A `key` attribute on the root, whose type is a name and not a built-in type
  such as `String` or `Long`. The type of the first such attribute is the
  aggregate's identity. A root without one is a problem.
- No `import`. A project records its domain in one file, so a domain file
  with an `import` is refused.
- A context name that arclint can use. A `name` directive wins. Otherwise a
  CML name that starts with a lower-case letter and holds only lower-case
  letters, digits and `_` is kept, and any other name becomes snake case:
  `CustomerManagementContext` is `customer_management_context` and
  `HTTPGateway` is `http_gateway`. Two contexts that end up with one name,
  or a name arclint cannot accept, are an error.

### What it takes

| in the file | in arclint |
|---|---|
| the first `Domain` | the project name (through its `name` directive when it has one); its `domainVisionStatement` is the description. With no `Domain`, the `ContextMap` name is the project and the description is empty. With neither, arclint asks you to add one. Subdomains and further `Domain` blocks are not read |
| every `BoundedContext`, in file order | the contexts. A context with `type = TEAM` is a team, not a bounded context, and is skipped together with the relationships that name it |
| an `Aggregate` in a context or in one of its `Module` blocks | an aggregate named for its root. Other entities in it are member entities. Entities anywhere else are not read |
| an operation `ensure<Key>` on a root or a `ValueObject`, with a doc string | an invariant. Without a doc string, or when the part after `ensure` is not a key, it is an ordinary operation. A key that only names the act of checking (`check`, `validate`) is an error, as in YAML |
| an operation `assert<Key>` on a root or a `Service`, with a doc string and an `arclint.on` hint entry | an assertion. Otherwise it is an ordinary operation |
| a `ValueObject` that declares `satisfiedBy`, `SatisfiedBy`, `satisfied_by` or `isSatisfiedBy` | a specification |
| every other `ValueObject` in the context | a value object |
| every `DomainEvent` in the context | an event. It is raised by the aggregate whose root names it in `belongsTo`; failing that, by the aggregate that contains it; failing that, by none. A `belongsTo` that names no root of the context is an error |
| a `Service` at the level of the context, a `Module` or an `Aggregate` | a domain service. A service in an `Aggregate` with the hint entry `arclint.role=factory` is that aggregate's factory instead |
| a `Repository` under the root, else the root's `arclint.repository` hint entry | the aggregate's repository |
| `arclint.alias` hint entries | aliases |
| `question` directives inside a context's braces | questions |

Relationships become relations in the order the map lists them, as the table
below shows. A relation's `From` is the upstream, or the first participant of
a symmetric relationship. A `description` directive directly above a
relationship is its description, and `separate-ways` directives in the map
body become separate-ways relations at their position.

| relationship in CML | arclint kind |
|---|---|
| Partnership | `partnership` |
| Shared Kernel, in any notation, including a bare `<->` | `shared_kernel` |
| Customer/Supplier, with or without ACL | `customer_supplier` |
| Upstream/Downstream with `ACL` | `anticorruption_layer` |
| Upstream/Downstream with `CF` (and no `ACL`) | `conformist` |
| Upstream/Downstream with `OHS` (and neither of those) | `open_host_service` |
| Upstream/Downstream with `PL` (and none of those) | `published_language` |
| Upstream/Downstream with none of these roles | `conformist` |

Two relationships for one pair of contexts are an error. So is a
relationship that names a context the file does not declare.

### What it leaves untouched

Everything else is outside arclint's language and stays in the file as it is:
subdomains, the map's type and state, relationship names,
`implementationTechnology`, `exposedAggregates`, `downstreamRights`, a
bounded context's `type`, `responsibilities`, `implements`, `realizes` and
`refines`, applications with their flows and coordinations, use cases, user
stories, stakeholders, value registers, attributes, references, plain
operations and ordinary comments. Enumerations, traits, data transfer
objects, command events and basic types are not arclint terms either.
Application services are application services; arclint does not read them as
domain services.

## Editing a CML file

`arclint domain define` and `arclint domain remove` work on a CML file the
way they work on a YAML one. They change what differs from what you ask for
and keep the rest:

- A command that changes nothing arclint records does not write the file.
  It stays the same, byte for byte.
- An edit changes only the lines of the element it edits in a file arclint
  wrote. In a file written by hand, the first edit also sets indentation,
  line breaks and blank lines to arclint's layout, once; the words you
  chose stay, including the optional ones such as the `=` after
  `domainVisionStatement`, `void`, and empty `()` on an operation.
- A changed element is edited where it sits. Its position, attributes,
  operations and hint entries that are not arclint's, and comments all stay.
- A new element goes where a fresh file would put it. A new context goes
  after the last `BoundedContext` and into the map's `contains` line. A
  context's `Module` is created when it is needed.
- A removed element removes its whole declaration. Removing a context also
  removes its `contains` entry and every relationship and `separate-ways`
  comment that names it. Removing an aggregate keeps the value objects,
  events and services its `Aggregate` block also held: they move to the
  context's `Module`, or for services to the context itself.
- A removal or a rename never leaves a name Context Mapper cannot resolve.
  References that may be left out are updated: a removed context leaves
  `realizes`, `refines`, `owner`, `Stakeholders of` and `ValueRegister for`,
  a removed aggregate leaves `exposedAggregates`, and a renamed context,
  aggregate or project is renamed wherever it is named. When a reference the
  grammar requires names it, such as a step of a `Coordination` or a
  `Flow`, the command refuses and names that construct and its line.
- A change of order moves the declarations, with their comments, to the
  order you ask for.
- An entity moved to another aggregate is the same declaration, attributes
  and all.
- A renamed repository keeps its body. When the new name cannot be a CML
  `Repository` (it must end in `Repository` and be unique in the file) and
  the block holds operations, the command refuses instead of dropping them.
- Only the `ensure` and `assert` operations that read as invariants and
  assertions are rewritten or removed. Other operations stay.
- A relationship that already reads as the relation you ask for is left as
  it is, including extra roles and its name. Otherwise only its head is
  rewritten to the form of the new kind; its name and
  `implementationTechnology` stay. When the new kind is symmetric,
  `exposedAggregates` and `downstreamRights` go.

After each edit arclint reads the edited file again and requires that it
records the language you asked for. It then formats the file and writes it
atomically.

`arclint domain export` into a file that already records a language follows
the same rules.

## Warnings in Context Mapper

A file arclint writes raises no error in Context Mapper. ArcLint's tests
check that against Context Mapper's own validators, and the files in this
repository were also checked with Context Mapper 6.12.0 itself. Context
Mapper can show these warnings, and only these:

- `The module '<context>' contains Services and/or Entities that are not
  part of an Aggregate.` ArcLint records value objects, specifications and
  events on the context, not on one aggregate. CML holds such objects in a
  `Module`, so arclint writes one, and Context Mapper warns about it.
- `Use - <Type>` on an identity attribute such as `RuleID id key`, when
  the context also records `RuleID` as a value object. ArcLint writes the
  identity as an attribute, because a reference from an aggregate to a value
  object kept in a `Module` would raise a different warning: that the object
  is not reachable from the aggregate.
- `The name '<Name>' is used for multiple domain objects.` Two contexts
  that each record a term of the same name write two declarations of it.
  ArcLint defines a term inside one context, so this is allowed there.
- `The domain object name should begin with an upper case letter` and
  `The service name should begin with an upper case letter`, for terms
  whose names start with a lower-case letter or are written as a handle.
