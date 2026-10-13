# Context Mapper syntax verdicts

`context_mapper_6_12_verdicts.json.gz` is a gzip-compressed JSON table of input,
name, accepted, and first syntax-error line. `TestContextMapperDifferential`
checks every row using Go alone. Accepted rows use line zero. Only syntax and
lexer diagnostics determine these verdicts; linking and domain validation are
outside the syntax package's contract.

The external oracle loaded each input with Context Mapper 6.12.0's Xtext parser,
resolved references, and ran `CheckMode.ALL`. Its two grammar files are identical
to `grammar/ContextMappingDSL.xtext` and `grammar/TacticDDDLanguage.xtext`.
No Java runtime, Maven dependency, or oracle invocation belongs to this package.

The table contains 45,649 distinct inputs:

- 45,196 generated grammar cases;
- 337 distinct inputs from every vendored corpus file (identical file contents
  share a row);
- 116 extracted documentation models, including incomplete and invalid models.

A grammar-tree walk covered 162 reachable unordered groups and alternatives in
both pinned grammars, using complete models rooted at ContextMappingModel. For
unordered groups it exercised every ordered member pair, contiguous repetitions,
and attempted resumption after another member. It also exercised every
permutation of all members of groups with at most five members, and every
permutation of the members with overlapping FIRST tokens in larger groups.
Prefixes and alternatives varied independently, including doc STRING, hidden
ML_COMMENT, abstract, visibility, ID, and shared keywords. All BoundedContext
Application/Module/Aggregate/Service block orders and documentation masks were
included. Shared-token sequential loop boundaries were tested in both orders,
with one and two repetitions. Alternatives inside member loops included domain
objects, attributes, references, operations, associations, relationship
notations, command/operation invocation, event production and state targets.
Each alternative-loop iteration selects independently, so every ordered pair
exercises every adjacent boundary. Unordered groups also remember which members
have been selected; full colliding-member permutations and attempted resumes
exercise that state.

The initial interpreter disagreed on 3,604 rows: 3,552 acceptance decisions and
52 rejection lines. BoundedContext accounted for 388, Application for 3,216.
After fixing selected-member continuation and missing required-member positions,
all 45,649 rows agree, including the first error line.

## Why the loop commits

The pinned evidence in
`AbstractAntlrGrammarWithActionsGenerator.xtend` (cited in `xtext_sources.md`), `_ebnf2(UnorderedGroup)`,
selects one member using `canSelect`, emits a repeated member as
`({true}?=> member)+`, then calls `returnFromSelection`. The helper in
`UnorderedGroupHelper.java` (cited in `xtext_sources.md`) prevents selecting that member again
once its contiguous block ends.

ANTLR distinguishes competing declarations when selecting a block. Inside the
selected repetition, however, its FIRST-token lookahead commits to another
member. In BoundedContext, SculptorModule, Aggregate and Service all start with
an optional doc STRING. Once Module or Aggregate has been selected, a following
doc STRING starts another iteration of that block. The subsequent Service
keyword fails at its own position, expecting Module or Aggregate. The same
mechanism applies to documented commands, events and services in Application.
The previous continuation-based interpreter incorrectly backtracked out of that
iteration into another unordered member. Ordinary alternative loops retain
their deeper lookahead behavior; they must not inherit this commitment.
