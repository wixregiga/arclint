# Refacoring rules

**DO NOT CONTINUE IF YOU DO NOT SATISFY AT LEAST ONE OF THE FOLLOWING CRITERIA:**
    - Are you being asked to refactor by a human or orchestrating agent? If yes, skip to overview section.
    - Are you are at the cleanup or refactor stage of a feature? If yes, skip to overview section.
    - Are you tasked with a spike? If yes, skip to overview section.
    - Are you on a refactor branch? If yes, skip to overview section.

## Overview
Orchestrating agents must follow this rule and implementing agents must receive this information from the orchestrator before beginning wide scale refactor efforts.

## References
References are from the root of the repository.

- Domain-Driven-Design Vocabulary: arclint/docs/design-vocab.yml
- Domain Vocabulary

## Rules
Below are the rules for refactoring that have been compiled from past agent failures and successes. Read each one. Peform the asks, the checks, do not ignore the rules, and generate the alerts as is necessary. Peform those steps at the beginning of the refactor. Peform these steps during the refactor as necessary. Always peform it at the the end of the refactor.

```yml
- avoid: Treating existing code as design authority
  applyTo: [{actor: agents, scope: all}] 
  reason: “The code cannot do this” describes migration cost it is not a reason to reject the design.
  rule: Evidence about what exists is never an argument about what should exist
  instead: 
    - try: Keeping two columns. "what exists" and  "what should exist". Lean on "what should exist".
  orchestrator: Keep the vision focused on what should exist and the desired end state of the program
- avoid: Diagnosing architectural issues at the shalow symptom level
  applyTo: [{actor: agents, scope: all}] 
  instead:
    - ask: “What concept lacks a clear owner?”
    - ask: "What makes this refactoring structurally difficult?"
    - check: the semantic rules first
    - check: that you are using the domain language
    - alert: on use of words that are not definied in the vocabulary document
- avoid: Under-using source material
  applyTo: [{actor: agents, scope: all}] 
  instead: Create a source checklist before design work and record which decisions each source informs.
- avoid: Assuming a blast radius
  applyTo: [{actor: agents, scope: all}] 
  instead: Test uncertain constraints with the smallest representative spike before defending them.
- avoid: Mistaking green gates for good modeling
  applyTo: [{actor: agents, scope: all}] 
  instead:
    - require: Give every domain term an owner, invariants, lifecycle, construction rules, and intended methods before planning implementation.
- avoid: Using DDD vocabulary without enforcing it
  applyTo: [{actor: agents, scope: all}] 
  instead:
    - try: Give every domain term an owner, invariants, lifecycle, construction rules, and intended methods before planning implementation.
```
## Reviews

### Anemic-model review
Before calling the model complete, ask:
  - Can outside code create an invalid domain object?
  - Are invariants enforced by the owning type?
  - Are domain operations methods, or mostly free functions manipulating data?
  - Does a DTO expose aggregate internals?
  - Would replacing the domain types with maps and strings preserve most behavior?

Verdict: Multiple “yes” answers mean the model needs another pass even if every test passes.