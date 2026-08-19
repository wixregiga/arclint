+++
title = "Patterns"
description = "What a Pattern is in ArcLint, and the current patterns command."
weight = 5
+++

A **Pattern** is a named, versioned collection of Rules packaged for
distribution. In the domain it may also carry Extensions, coverage,
vocabulary, and provenance so a repository can pin an exact release
and reason about where each Rule came from.

Patterns are the distributable form of Rules. Local Rules in
`rules.yaml` do not need a Pattern. When a Pattern is available, a
repository would reference a specific version rather than copying rule
text by hand.

## Current CLI

```bash
arclint patterns
```

Lists Pattern distribution packages the running CLI can see. Today the
command prints:

```
no patterns available
```

There are no built-in Pattern packages shipped with the CLI yet, and
the listing is empty until distributions exist and are discoverable.

`arclint init` drafts a starter `rules.yaml` from `--languages` (and
`--force` to overwrite). It does not install or select Patterns.

## What this page does not cover

Authoring layout, local pattern directories, namespaces, install flags,
and packaged test suites for Patterns are not part of the current
product surface. Prefer repository-local Rules (see [Rules](./rules.md)
and [Getting started](./getting-started.md)) until Pattern distribution
is available.
