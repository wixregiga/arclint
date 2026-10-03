# Building the ArcLint Domain Reviewer

These obligations are for ArcLint contributors. The reviewer's promises to
its caller are in [requirements.md](requirements.md).

DR-8. Contributors shall author the reviewer as editable instructions kept
separate from the installed domain guard.

DR-12. ArcLint shall ship the authored reviewer instructions as embedded assets
in the same release as the CLI.

DR-14. Contributors shall verify installation behavior through the application
and real host adapter, and evaluate the reviewer's behavior on supplied cases.

Record the meanings and product obligations before implementing them. Explain
which existing meanings justify the change, who owns each decision and what
the caller uses. An instruction file is an implementation: it still needs
review against its recorded purpose and obligations.

Keep one authored source for reviewer instructions. Installed host files are
copies, with the ArcLint release recorded separately. Preserve a person's
changes to an installed copy instead of silently overwriting them.

A CLI test can establish file installation, preservation and version reporting.
A behavioral evaluation can establish what the reviewer did for the supplied
case. Neither proves that every future review is correct or that a host has
activated the reviewer. Report these kinds of evidence separately.

Review the domain in ordinary language. Agent Instructions and AGENTS.md can
name real responsibilities without requiring a Go type for every word. A
technical helper does not need a new domain term when it only carries out an
already justified operation; a helper making a new product decision does need
an established owner and meaning.
