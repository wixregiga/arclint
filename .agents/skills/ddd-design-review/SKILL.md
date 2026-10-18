---
name: ddd-design-review
description: Beta. Review a plan, issue or diff for where each DDD decision and change lives, check it against arclint's sourced meta-model, declined rules and the rule for a service's assertions, steelman the opposite, and give a verdict per decision. Use before implementing or delegating a design on arclint, and when reviewing one.
---

# DDD design review (beta)

You review a design. You do not edit code or the domain recording; a change of meaning goes to the domain-librarian skill.

## Source standard

- Tier 1 grounds rules: Evans 2003 (Domain-Driven Design) and 2015 (DDD Reference, https://www.domainlanguage.com/wp-content/uploads/2016/05/DDD_Reference_2015-03.pdf); Vernon 2013 (Implementing Domain-Driven Design), his papers and his sample code (IDDD_Samples); Evans and Fowler 1997 (Specifications).
- Tier 2 grounds guidance: Fowler's Patterns of Enterprise Application Architecture, Fowler's bliki.
- Tier 3, blogs, only illustrates. No verdict rests on tier 3 alone.
- Excluded, never used: Goodreads, unknown Medium authors, Stack Overflow, AI summaries, search snippets.
- Canonical references are never removed or demoted.
- Every citation states its tier and whether it was read: `Evans 2015, p. 21 (tier 1, read)`. Read means you read the passage this session, or the meta-model cites it with `verification: text` or `fetched`; `carried` is not read. Never invent a page or a quote.
- A secondhand quote counts as the quoting author's: Evans quoted in AnemicDomainModel is `Fowler, AnemicDomainModel (tier 2, read)`.

## Where the grounds are

- `internal/domain/vocab/metamodel.arclint.yaml`: `works` is the bibliography; each building block's `invariants` are rules and its `guidance` is advice, each with sources.
- Declined rules: the meta-model's `declined` section (issue #144); until it merges, the list in `gh issue view 144`. A declined entry reopens only on new literature or a new owner decision, never on a request alone.
- A service's assertions: issue #149.
- `arclint context <paths>` and `arclint domain` for the zones and the recorded domain.

## Procedure

1. **Name the building blocks.** For each concept and operation in the input, name its meta-model term (aggregate, entity, value_object, domain_service, application_service, repository, factory, specification, domain_event) and where its decision and its changes live, as file or layer.
2. **Check the placement** against the invariants and guidance of each block, the declined rules, and these shapes for a guarantee (assertion):
   - Aggregate root or domain service: the operation the assertion protects calls `Assert<Key>`, a method of the owner or a function of the context, in its own body (`assertion/checked-by-its-operation`; owner decision, issue #149). A check a caller is expected to make, a returned value object, or tests under `verified_by` do not count.
   - Service declared as an interface (Go interface, TypeScript interface or abstract class, Python ABC or Protocol): it has no body to call the check from, so the rule leaves its assertion undetermined. How such a service is judged is open for the owner.
   - An effect the domain cannot see, such as "a refusal changes nothing on disk", is proved by an automated test (Evans 2015, p. 22).
   - A proposed rule that matches a declined entry conflicts; cite the entry's id, reason and basis.
3. **Steelman the opposite.** For each recommendation, state the strongest case against it, from tier 1 or 2 sources where they exist. Keep the recommendation only when that case fails on a source or a recorded owner decision; say which. Otherwise the decision is undecided.
4. **Report** one entry per decision:

```
Decision: <one sentence>
Block and home: <meta-model term>, <file or layer> (decides | carries out)
Verdict: conforms | conflicts | undecided
Basis: <invariant, guidance or declined id>; <work, page> (tier N, read | not read)
Opposite: <strongest case against> -> <why it fails, or why it stands>
Owner question: <the product question, for undecided only>
```

Conflicts names the source it conflicts with. Undecided names the one product question the owner must answer. Mark claims you did not verify `[INFERENCE]`.
