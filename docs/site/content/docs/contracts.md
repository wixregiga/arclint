+++
title = "Domain Contracts"
weight = 50
+++

# Visible Domain Contracts

ArcLint now supports mapping your Ubiquitous Language directly to **Visible Domain Contracts**. This ensures that the expert-defined invariants, assertions, and specifications recorded in your domain vocabulary are continuously evaluated against your codebase via exact declaration and call facts.

A Rule whose assertion is `invariants` enforces that these structural contracts remain unbroken as the code evolves:

```yaml
rules:
  entities/contracts-visible:
    description: "Every aggregate's invariants are visible through its cluster method."
    on: entities
    invariants: {}
```

## Recording Domain Contracts

You can record domain contracts in your `ubiquitous-language.yaml` library file under the respective bounded context. We support three categories of contracts:

### 1. Cluster Invariants
Must-always/must-never rules that hold at all times within a context. A cluster invariant on an **Aggregate** requires a named contract (e.g., `published-frozen`). The invariant method must be called from the entity's constructor and every exported command.

```yaml
invariants:
  - statement: "A published Event never changes."
    owner: "Event"
    id: "published-frozen"
```

*Example in Go:*
```go
type Event struct {
    // ...
}

func NewEvent() (*Event, error) {
    e := &Event{}
    if err := e.PublishedFrozen(); err != nil {
        return nil, err
    }
    return e, nil
}

func (e *Event) PublishedFrozen() error {
    // Invariant logic
    return nil
}
```

### 2. Assertions
Post-conditions of a named operation. Unlike invariants, an assertion holds when a specific operation occurs, rather than at all times.

```yaml
assertions:
  - statement: "Every TicketTier has a Price before publish."
    owner: "Event"
    id: "tiers-priced"
    on: "Publish"
```

*Example in TypeScript:*
```typescript
class Event {
  // ...
  publish(): void {
    this.tiersPriced(); // Assertion call
    // ...
  }
  
  tiersPriced(): void {
    if (this.tiers.some(t => t.price <= 0)) {
      throw new Error("Tier unpriced");
    }
  }
}
```

### 3. Specifications
Named predicates passed around by domain experts. A specification type must carry a satisfaction method (e.g., `SatisfiedBy`). 

```yaml
specifications:
  - name: "HighValueOrder"
    definition: "An order the house treats as high value."
```

*Example in Go:*
```go
type HighValueOrder struct{}

func (h HighValueOrder) SatisfiedBy(o Order) bool {
    return o.Total() > 1000
}
```

## Evaluation & Facts

When checking conformance (e.g. via an `invariants` rule like `entities/contracts-visible` above), ArcLint utilizes deep language facts, specifically **Declarations** and **Calls**, to verify structural compliance. `invariants: {closed: true}` tightens the posture: every exported error-returning function in the owner's files must call the cluster method, not only the constructor and commands.

The engine verifies that:
- **Cluster Invariants:** The constructor and all domain commands call the invariant method.
- **Assertions:** The specified operation (e.g., `Publish`) makes a distinct call to the assertion method (e.g., `TiersPriced`).
- **Constructors:** Value objects properly enforce value integrity at construction.

These checks are powered by internal matrices (`check_invariants.go` and `contract_matrix.go`), ensuring exact method resolution.

### `SatisfiedBy` Specifications

For Specifications, the engine expects the specification type to provide a satisfaction method. The engine searches the type's declarations for an Evans satisfaction method:
- `SatisfiedBy` (Go)
- `satisfiedBy` (TypeScript)
- `satisfied_by` (Python)

We only support Go, Python, Typescript right now. If a specification lacks one of these methods, ArcLint flags it as a violation ("missing satisfaction method").

## Schema Formatting & Tooling

To aid in authoring, the JSON schema backing the ubiquitous language library (`library.schema.json`) has been rigorously structured. 

**Hover Text Enhancements:** The schema documentation strings are now cleanly split into short paragraphs and lists. When using compatible IDEs, hovering over a field (like `invariants` or `assertions`) provides highly readable, structured guidance directly in the editor.

**Strict Validation:** The schema design adheres to strict standards enforced by Spectral linting. This guarantees your domain language file remains pristine, canonical, and accurately parseable by the contract evaluation engine.
