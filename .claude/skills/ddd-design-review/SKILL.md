---
name: ddd-design-review
description: Beta. Review a plan, issue or diff for where each DDD decision and change lives, check it against arclint's sourced meta-model, declined rules and declared-service shapes, steelman the opposite, and give a verdict per decision. Use before implementing or delegating a design on arclint, and when reviewing one.
---

# DDD design review (beta)

You review a design. You do not edit code or the domain recording; a change of meaning goes to the domain-librarian skill.

## Source standard

- Tier 1 grounds rules: Evans 2003 (Domain-Driven Design) and 2015 (DDD Reference, https://www.domainlanguage.com/wp-content/uploads/2016/05/DDD_Reference_2015-03.pdf); Vernon 2013 (Implementing Domain-Driven Design), his papers and his sample code (IDDD_Samples); Evans and Fowler 1997 (Specifications).
- Tier 2 grounds guidance: Fowler's Patterns of Enterprise Application Architecture, Meyer's Object-Oriented Software Construction, Wlaschin's Domain Modeling Made Functional, Fowler's bliki.
- Tier 3, blogs, only illustrates. No verdict rests on tier 3 alone.
- Excluded, never used: Goodreads, unknown Medium authors, Stack Overflow, AI summaries, search snippets.
- Canonical references are never removed or demoted.
- Every citation states its tier and whether it was read: `Evans 2015, p. 21 (tier 1, read)`. Read means you read the passage this session, or the meta-model cites it with `verification: text` or `fetched`; `carried` is not read. Never invent a page or a quote.
- A secondhand quote counts as the quoting author's: Evans quoted in AnemicDomainModel is `Fowler, AnemicDomainModel (tier 2, read)`.

## Where the grounds are

- `internal/domain/vocab/metamodel.arclint.yaml`: `works` is the bibliography; each building block's `invariants` are rules and its `guidance` is advice, each with sources.
- Declined rules: the meta-model's `declined` section (issue #144); until it merges, the list in `gh issue view 144`. A declined entry reopens only on new literature or a new owner decision, never on a request alone.
- Declared-service shapes: issue #148. Decide-then-carry-out: issue #128.
- `arclint context <paths>` and `arclint domain` for the zones and the recorded domain.

## Procedure

1. **Name the building blocks.** For each concept and operation in the input, name its meta-model term (aggregate, entity, value_object, domain_service, application_service, repository, factory, specification, domain_event) and where its decision and its changes live, as file or layer. The domain decides; the application carries out (issue #128):
   - The deciding operation receives what it needs as values, returns its verdict as a value, and changes nothing (Evans 2015, p. 21). A verdict that carries a rule is a value object.
   - The deciding operation derives what it expects, such as who must take part, from the values it receives. An assertion that checks only an expectation the caller chose constrains nothing; a service contract is "a set of assertions about interactions with the service" (Evans 2015, p. 14).
   - The application gathers the values, asks for the verdict, and makes the changes only with that verdict in hand. It holds no business rule: if deleting its steps would lose a business decision, that decision is misplaced (application_service guidance; Evans 2015, p. 10).
   - Flag a domain operation that takes callbacks or functions to perform changes, or does I/O: it is application workflow moved into the domain (Evans 2015, pp. 10 and 21).
2. **Check the placement** against the invariants and guidance of each block, the declined rules, and these shapes for a guarantee (assertion):
   - Aggregate root or concrete service: the owner declares `Assert<Key>` and the operation calls it (`assertion/checked-by-its-operation`).
   - Service declared as an interface (Go interface, TypeScript interface or abstract class, Python ABC or Protocol; Evans 2015, p. 14: "a standalone interface declared as a service"): the guarantee is checked once, in the model, by (a) a function of the context spelled from the key (`AssertNeverExceedsTotal`), as the `MD5EncryptionService` of Vernon's IDDD_Samples calls the shared `AssertionConcern` checks, or (b) the operation returning a recorded value object of the same context whose invariant has the same key, enforced at its constructor (Evans 2015, p. 21). Never an interface method, a callback, or prose.
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
