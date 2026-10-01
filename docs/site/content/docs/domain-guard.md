---
title: Domain guard for OMP and Codex
description: Install scoped domain review using supported local agent hooks.
---

Run setup from the project root:

```sh
arclint agents hooks --host omp
arclint agents hooks --host codex
# Repeat --domain for additional recording files:
arclint agents hooks --host codex --domain ubiquitous-language.yaml
```

Setup installs `.arclint/domain-guard.json` and the selected host's files:
`.omp/extensions/arclint-domain-guard/`, or `.codex/hooks.json` and
`.codex/hooks/arclint-domain-guard/guard.py`. It works without a ruleset.
It preserves existing host configuration and unrelated hooks, and does not
configure credentials or grant trust. An identical installation is a no-op;
modified installed guard files are reported as conflicts and preserved.

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

The Codex installer targets Linux/WSL projects. It includes a Windows command
override invoking Python in the installation's WSL distribution. Python 3 and
a compatible Codex CLI must be available there. Native Windows-only project
installation is not supported.

Coverage is explicit. `domainFiles` lists domain recordings; optional
`sourcePatterns` selects implementation evidence:

```json
{
  "version": 1,
  "domainFiles": ["domain.arclint.yaml"],
  "sourcePatterns": ["internal/domain/rule/*.go"]
}
```

Source patterns must have a literal project-relative prefix. They support
`*`, `?` and `**`; missing matches and paths escaping the project fail review.
The guard does not follow recording references automatically, infer a complete
domain boundary, or review the whole repository. Selected application or
delivery files can provide evidence for a recorded obligation without becoming
domain-layer code. Every supplied recording and source file contributes to the
review fingerprint. Keep scope within 256 files and 300 KB.

Before edits, review checks proposed domain work against the original request
and existing model, including unjustified categories and responsibilities.
After changes it checks missing promised concepts, classification, ownership
and duplicated decisions. The no-comments check applies to recordings only.
Domain-related responses receive review even without file edits. Findings need
an exact supplied quote, a concrete conflict and a scoped correction. Repairs
and evidence-backed rebuttals receive fresh review; the request is not rewritten.

Known edit and shell tools receive pre-edit review during domain work, and
explicitly referenced scoped files trigger review before a domain-work flag
has been set. Obscured paths or custom tools can escape pre-edit recognition;
changed scoped files are checked after tool results and again at completion.
This is a workflow guard, not a security boundary against a hostile agent,
a replacement for functional tests, or proof that omitted code is correct.

Changes to scope configuration, installed guard code, and (for Codex) hook
definitions invalidate the session snapshot. So do changes to
`rules.arclint.yaml`, `rules.yaml`, `.arclint/baseline.v2.json` and
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

These tests inject review verdicts and cover repair, source freshness,
response-only review, governance changes, failures, operator escape and
installation preservation. They do not establish real model review quality
or native execution of trusted Codex hooks.
