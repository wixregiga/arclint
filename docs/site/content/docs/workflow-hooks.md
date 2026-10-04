---
title: Task-focused workflow hooks
weight: 8
description: Report domain workflow departures in the coding agent's current task.
---

ArcLint's workflow hook helps a coding agent follow the project's domain
workflow while it works. It reports departures with quoted evidence and a
suggested correction: reading source before obtaining context, implementing
changed meanings before explaining and recording them, missing enforcement,
misplaced ownership or unjustified duplication. It also reassesses repairs and
rebuttals.

This is a public feature for any project using ArcLint. ArcLint uses the same
feature for its own development.

## Install in your project

From the project root, run:

```sh
arclint agents workflow install
```

The current host integration is Codex. Installation adds native hooks to
`.codex/hooks.json`, preserving existing hooks, and installs the executable and
editable review instructions under `.codex/hooks/arclint-workflow-guard/`.

Start Codex with the installed launcher:

```sh
bash .codex/hooks/arclint-workflow-guard/start-codex.sh
```

For an ordinary checkout, it starts Codex in the project with normal
configuration. Linked-worktree launchers supply an identical, sorted inventory
of the exact workflow-hook definitions installed across sibling worktrees,
alongside inherited hooks. Only the provider belonging to the current worktree
reviews its events; the others do nothing. The shared definitions avoid
conflicting trust records between worktree sessions. The launcher does not edit
the main checkout's hooks.

After adding or removing a sibling workflow installation, rerun
`arclint agents workflow install` in previously installed worktrees to refresh
their launchers. Changed definitions require Codex's normal trust review again.

Installation and launching do not grant trust. In the launched session, inspect
the definitions with `/hooks` and follow Codex's trust steps. Then exit and run
the launcher again to start a fresh session. Verify the loaded definitions and
actual review activity separately. Desktop hook settings are another way to
inspect host trust, but opening an existing desktop session is not equivalent
to starting this launcher.

Check the installed files with:

```sh
arclint agents workflow status
```

Status describes installation and integrity. It does not establish that Codex
has activated or invoked the hook. Installation preserves unrelated hooks and
does not replace independently installed tools.

## What the agent receives

The hook reports feedback through native events while the agent works and when
it finishes. A finding identifies the supplied passage, quotes the relevant
text, explains the departure and suggests a correction. Missing history or
unavailable evidence appears as a coverage limit. Reports are advisories;
they do not block tools or grant approval.

Session reports are also recorded under
`.arclint/cache/workflow-guard/reports/`. They show what was reported and include
unavailable-review messages; a saved report alone does not prove a correct review.

Scope follows the current task and observed actions. The collector supplies
available project guidance, task-mentioned files, paths in native file tools
or patches, and changes observed since the task began. Existing unrelated
changes are not automatically attributed to that task. Available nested
`AGENTS.md` instructions accompany affected files. Source text is not restricted
to Go or TypeScript, and the hook imposes no comment ban.

The evidence is bounded. Missing files, unreadable or binary content,
truncation and unavailable earlier actions limit the review. A present-day
snapshot cannot establish whether a decision preceded its implementation.
Semantic judgment can be mistaken: review the quoted evidence and provide a
repair or concrete rebuttal. An empty findings list is not certification of the
whole repository.

## Review supplied evidence directly

Other integrations can send a task and named text passages to the same public
review operation:

```sh
arclint agents workflow review <<'JSON'
{
  "task": "Review the proposed change to the booking workflow.",
  "passages": {
    "proposal": "Add a second booking policy without checking the existing policy."
  },
  "limits": ["The existing domain recording and source were not supplied."]
}
JSON
```

The result contains `Findings` and `Limits`. Each finding contains `Evidence`,
`Quote`, `Departure` and `Correction`. The same domain operation validates the
grounding of native-hook reports and direct reviews; model or malformed-response
failures are reported as unavailable. Running semantic review requires the
configured Codex review process to be available and authenticated.
