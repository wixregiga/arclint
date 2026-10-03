---
title: Domain guard for OMP and Codex
weight: 9
description: Install scoped domain review using supported local agent hooks.
---

A hook connects ArcLint's domain guard to a supported point in an agent's
work, so the agent can receive domain guidance and review feedback while it
builds. The connection, the review, and installation have different jobs.

| Term | Meaning and responsibility |
| --- | --- |
| Hook | A connection between a supported host event and the supplied domain guard. Its definition describes when the host invokes the guard. |
| Agent host | The coding environment that provides the events and runs the hook, currently OMP or Codex. The host controls loading, activation and any required trust. The command calls this `--host`. |
| Domain guard | The supplied review behavior over configured domain recordings, selected source evidence, and domain-related proposed changes or responses. It returns concerns with evidence and scoped repair guidance. |
| Install hooks | The application operation that places the connection files and review configuration in a project. The host-specific installer supplies the appropriate file format and locations. |

Installation preserves existing settings and unrelated hooks. Repeating an
identical installation has no effect; conflicting installed guard changes
are reported without overwriting them. Installing files does not establish
that the host has loaded or trusted them.

A guard finding is feedback from reviewing selected evidence. Completing a
guard review means that its current checks completed without an outstanding
finding; it does not establish conformance for every architectural obligation.
The existing Rules and conformance assessment retain their own meaning,
including evidence strength and uncertainty.

`InstallAgentHooks` coordinates setup in the application layer through
`AgentHooksInstaller`. The OMP and Codex infrastructure adapters implement that
port; the composition root chooses the adapter, and the CLI translates the
user's request. Host events, command definitions and installed files remain
integration details. The recorded adoption term `Installation` continues to
mean extending a Pattern; hook setup does not change that term's meaning.

Run setup from the project root:

```sh
arclint agents setup --host omp
arclint agents setup --host codex
# Repeat --domain for additional recording files:
arclint agents setup --host codex --domain ubiquitous-language.yaml
```

Setup installs `.arclint/domain-guard.json` and the selected host's files:
`.omp/extensions/arclint-domain-guard/`, or `.codex/hooks.json` and
`.codex/hooks/arclint-domain-guard/guard.py`. Setup creates a starter ruleset and an empty recording only when absent.
Use --pattern and --languages only when creating the ruleset; existing policy is
preserved. A custom --domain must already exist. Setup also publishes the
domain-librarian skill and a compact managed AGENTS.md pointer.
The lower-level agents hooks command remains available without a ruleset.
It preserves existing host configuration and unrelated hooks, and does not
configure credentials or grant trust. An identical installation is a no-op;
modified installed guard files are reported as conflicts and preserved.
Setup validates hook scope, host support and existing managed-file conflicts
before creating its rules, recording or guidance files. A rejected validation
leaves those files untouched. This does not promise crash-atomic filesystem writes.

Setup, hooks installation, guard status, reviewer installation and reviewer status
honor `--format json` through the normal report renderer. Their structured results
describe installed paths, scope and integrity; they cannot report host activation.

For OMP 18.4.4, start a new session or run `/reload`, then use
`/arclint-domain-status` to check loading, scope and the last review.

For Codex 0.159.2, open a new local session in the project, review and trust
the five ArcLint definitions through the CLI's `/hooks` browser, then start
a new session so SessionStart establishes the review snapshot. Untrusted hooks
are skipped. Project-local configuration must also be trusted by that host.
Desktop and WSL can use different user configuration and trust stores:
a desktop project opened through a WSL UNC path may require its own project
trust approval before hooks are even discovered. Do not infer activation from
files existing on disk. Use the host's hook listing to confirm discovery and
trust. Desktop 26.928.2636.0 ships Settings → Coding → Hooks, a From Projects
section, and per-hook Trust controls. Its folder-opening flow includes
"Trust this folder?" and "Trust folder"; explicitly untrusted folders instead
retain restrictions. These controls were verified in bundled UI code, not an
interactive activation walkthrough. The `/hooks` command is CLI-specific. Cloud-orchestrated sessions are unsupported.

Codex 0.159.2 loads linked-worktree hook definitions from the main checkout.
Writing a worktree-local hooks.json does not activate it in that version. The
shipped guard uses the event's session directory and that checkout's own explicit
scope, with separate session state; missing checkout configuration is unavailable
review. This routing does not update an older guard installed in the main checkout.
An independent new command definition requires the host's ordinary trust approval.
Do not replace a separate main installation or bypass hook trust for a test pass.

The Codex installer targets Linux/WSL projects. It includes a Windows command
override invoking Python in the installation's WSL distribution. Python 3 and
a compatible Codex CLI must be available there. Native Windows-only project
installation is not supported.

Coverage is explicit. `domainFiles` lists domain recordings; optional
`domainSourcePatterns` selects Go domain-source subjects; `sourcePatterns`
selects supporting implementation evidence. Setup accepts repeatable
`--domain-source` and `--source` flags for these fields:

```json
{
  "version": 1,
  "domainFiles": ["domain.arclint.yaml"],
  "domainSourcePatterns": ["internal/domain/rule/*.go"],
  "sourcePatterns": ["internal/application/assess_conformance.go"]
}
```

Source patterns must have a literal project-relative prefix. They support
`*`, `?` and `**`. Paths escaping the project fail review using the host platform's
path semantics. Missing configured material remains required evidence and prevents
a completion pass.
The guard does not follow recording references automatically, infer a complete
domain boundary, or review the whole repository. Selected application or
delivery files can provide evidence for a recorded obligation without becoming
domain-layer code. Every supplied recording and source file contributes to the
review fingerprint. Keep scope within 256 files and 300 KB.

Before edits, review checks proposed domain work against the original request
and existing model, including unjustified categories and responsibilities.
After changes it checks missing promised concepts, classification, ownership
and duplicated decisions. The comment check applies to recordings and explicitly selected Go domain source. It preserves required directives and legal headers; context-only source retains its comments.
Domain-related responses receive review even without file edits. Findings need
an exact supplied quote, a concrete conflict and a scoped correction. Repairs
and evidence-backed rebuttals receive fresh review; the request is not rewritten.

The pre-tool hook reviews a directly inspectable proposal to restore missing
scoped material. Permission to make that repair does not approve its contents:
the resulting files and completion response must receive fresh review. A mixed
patch that also changes unrelated or protected files gets no restoration exemption.
Malformed scope and escaping paths are errors, not missing material. Dedicated
inspection can report the problem without renewing an approval; arbitrary shell
commands are not presumed read-only.

Known edit and shell tools receive pre-edit review during domain work, and
explicitly referenced scoped files trigger review before a domain-work flag
has been set. Obscured paths or custom tools can escape pre-edit recognition;
changed scoped files are checked after tool results and again at completion.
This is a workflow guard, not a security boundary against a hostile agent,
a replacement for functional tests, or proof that omitted code is correct.

Changes to scope configuration, installed guard code, and (for Codex) hook
definitions invalidate the session snapshot. So do changes to
`rules.arclint.yaml`, `rules.yaml`, `.arclint/rules.yaml`, `.arclint/baseline.v2.json` and
`.arclint/baseline.json`, including creation of previously absent files.
Other rule/baseline paths are not automatically discovered. Restore unintended
changes; separately validate intentional policy changes before starting a new
session or reloading OMP. A new session establishes a new snapshot; this does
not independently certify a policy change.

OMP reviews use `runEphemeralTurn` with tools disabled and a 22-second deadline.
Codex runs a separate ephemeral CLI review with existing ChatGPT authentication,
user configuration ignored, a read-only sandbox, hooks/shell/multi-agent/code
mode disabled, and an empty temporary working directory. Its prompt supplies
only selected evidence; web search is disabled. The child has a 70-second
deadline within the hook's 90-second timeout. No service or new credentials
are installed. Reviews consume model usage and remain fallible. Native Codex
prompt/agent hook handlers are not used because this version skips them.

Failed, unavailable, malformed or stale review results do not grant approval.
OMP's completion handler checks the fresh verdict quickly; Codex's Stop command
performs its review within the command timeout. Three unsuccessful completion
attempts pause without approval. Codex startup/storage failures pause immediately
because retry counts cannot safely persist. Host-level failures before the
script runs, or untrusted/disabled hooks, cannot be made blocking by the script;
Codex can continue after some hook execution errors. Check host errors and
activation before relying on enforcement.

For a false positive, give the builder the conflicting quote and supporting
evidence for a scoped correction or rebuttal, then retry review. You can
interrupt a stalled turn. After the retry limit, investigate the reported
failure and start a new turn/session when ready; a paused turn is not approval.
Do not disable rules, edit baselines, or weaken scope to manufacture a pass.
Findings are deduplicated and stored per session in
`.arclint/cache/domain-guard/` (OMP) or
`.arclint/cache/codex-domain-guard/` (Codex). Keep these caches out of version
control. Stored reports are audit records, not independent attestations.

Offline verification:

```sh
go test ./internal/infrastructure/agents/omp ./internal/infrastructure/agents/codex
go test ./cmd/arclint -run 'Test.*(Hook|Install|Guard|Codex|OMP)'
node --test internal/infrastructure/agents/omp/assets/guard.test.mjs
python3 internal/infrastructure/agents/codex/assets/guard_test.py
```

These tests inject review verdicts and cover pre-tool repair permission, source freshness,
response-only review, governance changes, failures, operator escape and
installation preservation. They do not establish real model review quality
or native execution of trusted Codex hooks.

## Setup and scope contracts

Agent setup is an application operation coordinating the existing initialization,
guidance, skill and hook installers. It does not introduce another domain kind
or evaluator category. Existing inward-dependency and composition Rules apply.

The setup checks require preservation of existing rules and recordings, a
repeatable install, and refusal to replace user-modified guard files. Setup must
report activation separately from installation. Its managed AGENTS block points
to the existing domain-librarian skill and ArcLint context instead of copying
that protocol.

Review subjects are recordings plus explicitly selected domain source.
Supporting source is evidence, not automatically subject to the no-comments
policy. Go comments must be distinguished from quoted strings, runes and raw
strings. Compiler directives, cgo preambles, generated-file markers and legal
headers remain intact. The recording's leading schema directive is also kept.
Unsupported domain-source languages and unreadable scope are unavailable review,
never a passing verdict. Contract tests exercise failure, repair and freshness.

## Local packaging and updates

Setup uses the hosts' supported project discovery: .agents/skills for the
existing skill, .omp/extensions for the OMP extension, and .codex/hooks.json for
Codex command hooks. No public marketplace, account, service or credentials are
needed. This is a local native integration, not a published Codex plugin.
Do not additionally install duplicate plugin hooks: matching native hooks all run.

Repeat the same setup command to update. A receipt in .arclint/agent-assets.json
allows replacement only of unchanged assets ArcLint installed. The original
draft's known guard versions can migrate once. User-modified files stop the
update for review; there is no force-overwrite switch. Setup preserves unknown
configuration and existing scope unless source flags explicitly select it.
Updates invalidate loaded review snapshots; reload/new session is required.
Use arclint agents status for installed scope and changed/missing assets. It
cannot infer whether a running host loaded or trusted those files.

Existing project-authored librarian skills and vocabulary are preserved. Setup
writes the shared ArcLint review workflow to the companion ARCLINT.md and points
to both from AGENTS.md. It does not migrate an older domain recording or replace
its classification protocol merely to install hooks.

A legacy .arclint/rules.yaml is preserved, not masked by a fresh empty ruleset.
The status command identifies this case: the current structural checker requires
rules.arclint.yaml, so migration is separate work. Hook review still operates on
the explicitly selected recording and source. A project-local verified binary
may be used as ./.arclint/bin/arclint without replacing the user's global CLI.
