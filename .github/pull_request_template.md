<!--
  One PR = one issue. No drive-by formatting or second feature.
  Title must match the squash commit subject:

    <area>: <imperative summary>

  Example:  rules: add a depends_on pattern
  After squash, this title is what lands on main.
-->

# What changed

One or two sentences. Same claim as the commit subject, in imperative voice.

# Issue

Fixes #

# Area

<!-- rules | patterns | cli | config | output | vocabulary | extensions | agents | docs | ci -->

# User-facing?

- [ ] Yes: CLI, rule syntax, patterns, vocabulary, output, or docs a user follows. Release note draft is required.
- [ ] No: internal only. Do not add a release note.

# Release note draft

<!--
  Required if user-facing. One or two sentences for the notes.
  "Add support for …"  /  no "now"  /  no stack traces.
  Copy this onto the issue if the issue draft is stale.
-->

# Which problem it solves

Match the issue, not a new essay. If this disagrees with the issue, update the issue first.

# How it is tested

- [ ] `make ci` passes locally (vet, tests, and selfcheck).
- [ ] Rule behavior changes include cases for `arclint rules test`.
- [ ] I ran the "Success looks like" / expected-vs-actual steps from the issue on this build.
- [ ] The release note draft above is what I actually observed (or this PR is internal-only).

# Out of scope

<!-- What you did *not* change. Call out any formatting you had to touch and why it was required. -->