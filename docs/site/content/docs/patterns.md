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

Lists Pattern distribution packages the running CLI can see: built-in
packages embedded in the binary, then any local packages under
`.arclint/patterns`. Built-in packages appear the same way local ones
do; they differ only in where their bytes come from.

```
arclint/vertical@0.1.0  16 rule(s)  5 extension(s)  coverage [go]
```

`arclint init --pattern vertical` materializes that built-in package as
the repository `rules.yaml` and copies its Extension entries into
`.arclint/extensions`. Installed files become repository-owned copies.
`--pattern bare` (the no-flag default) writes the commented single-module
draft and no extensions. Local Pattern bundles may carry Extensions and
remain listable, but init still materializes built-ins only. Local
distribution files keep their `pattern:` header.

## What this page does not cover

Authoring layout, local pattern directories, namespaces, install flags,
and packaged test suites for Patterns are not part of the current
product surface. Prefer repository-local Rules (see [Rules](./rules.md)
and [Getting started](./getting-started.md)) until Pattern distribution
is available.
