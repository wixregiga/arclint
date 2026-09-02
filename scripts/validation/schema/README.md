# Schema Validation Functions

Custom Spectral functions for linting ArcLint's JSON Schemas (Draft 2020-12). These functions enforce schema consistency, documentation hygiene, and validation soundness.

## Functions Overview

### Reference & Definition Hygiene
- **`ref-description-match.js`**: Enforces that all `$ref` usages declare a sibling `description` that exactly matches the referenced definition's `description` (resolved via RFC 6901 pointers).
- **`one-of-ref-description-match.js`**: Specialized evaluator ensuring every branch in a `oneOf` referencing a `$def` provides a matching sibling description.
- **`no-orphaned-defs.js`**: Scans the document AST to ensure every named definition under `$defs` is referenced at least once.

### Structural & Type Integrity
- **`array-unique-items.js`**: Enforces `uniqueItems: true` on collection-like array schemas (paths, globs, rules) to prevent accidental duplicate entries in user configuration files.
- **`prevent-doubles.js`**: Checks literal arrays within the schema itself (`enum`, `required`, `examples`, `default`) to prohibit duplicate values.
- **`property-type.js`**: Ensures property schemas explicitly declare a `type` unless composed through `$ref`, `oneOf`, `anyOf`, or `allOf`.
- **`properties-object.js`**: Ensures any schema defining a `properties` dictionary explicitly specifies `"type": "object"`.
- **`combinators.js`**: Verifies that `oneOf` and `anyOf` combinators declare at least 2 subschemas to prevent degenerate constructs.
- **`dynamic-maps.js`**: Flags dynamic map objects (where `additionalProperties` is a subschema) that lack a `propertyNames` constraint.
- **`required-no-default.js`**: Flags mandatory (`required`) properties that also specify a `default` value, preventing ambiguous optional vs. required expectations.

### Documentation & Text Formatting
- **`description-formatting.js`**: Validates sentence formatting (initial capitalization, terminal punctuation, blank lines preceding Markdown lists, rejection of run-ons and punctuation glitches like `-.`).
- **`property-description.js`**: Flags non-ref property declarations that lack hover descriptions (`severity: warn`).
- **`no-whitespace-drift.js`**: Prohibits non-breaking spaces, leading/trailing whitespace, and consecutive spaces in titles, descriptions, and enum strings.

---

## Backlog & Deeper Evaluation Candidates

The following areas require deeper investigation or domain alignment when time permits:

1. **`additionalProperties: false` on Fixed Parameter Objects** (`object-additional-properties-defined`):
   - Currently set to `severity: warn`.
   - *Consideration*: Fixed parameter objects (such as extension invariants) should ideally disallow unrecognized keys, but closing bounds might break downstream extension payloads or forward-compatibility. Requires confirmation from domain owners before strict enforcement.

2. **Raw Unicode Escapes in Descriptions** (`\u003c` / `\u003e`):
   - Postponed due to potential generator coupling.
   - *Consideration*: Descriptions currently contain escaped angle brackets from Go's standard `json.Marshal`. Evaluate whether schema generation pipeline should disable HTML escaping (`SetEscapeHTML(false)`) or if Spectral should normalize and allow them.

3. **String `minLength: 1` Constraints** (`string-fields-min-length`):
   - Currently set to `severity: info`.
   - *Consideration*: Free-form string properties (like module descriptions) currently permit empty strings `""`. Determine whether empty descriptions are valid application states or authoring omissions.

4. **Regex Pattern Anchoring & Syntax Verification**:
   - Deferred for future evaluation.
   - *Consideration*: Validating that schema regular expressions in `pattern` keywords are fully anchored (`^...$`) to avoid partial substring matching surprises in client validators.
