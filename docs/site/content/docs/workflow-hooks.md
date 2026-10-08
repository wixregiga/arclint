---
title: Workflow hooks
weight: 8
description: Advise Claude Code and Codex when observed work skips a step of the domain workflow.
---

Coding agents often change code before they look up the architecture or
record the meaning a change introduces. ArcLint's workflow hooks watch what
the agent does and advise when it skips a step of the project's workflow:

1. Run `arclint context <paths>` on the files you will read or change,
   before opening them.
2. Decide whether the work introduces or changes a meaning. If it does,
   record it in `domain.arclint.yaml` first, using the domain-librarian
   skill. If it does not, say so in one sentence before editing.
3. Implement the change inside the zones `arclint context` reported, and
   keep existing behavior intact unless the task changes it.
4. Run `arclint check .` and the project's tests before finishing. Fix the
   findings in the code you changed, including baseline findings there;
   never weaken rules, baselines or exclusions to clear a finding.

The same four steps open the generated `AGENTS.md` block, and the hooks
repeat them when a session starts; both come from one source in ArcLint.

The hooks advise; they never block a tool call. Each reminder is given once,
so repeating an action does not repeat the reminder. Any project using
ArcLint can install them, and ArcLint uses them for its own development.

## Install

From the project root:

```sh
arclint agents workflow install
arclint agents workflow status
```

Installation writes the same hook command, `arclint agents workflow event`,
into each host's project configuration:

| Host | File | Events |
|---|---|---|
| Claude Code | `.claude/settings.json` | `SessionStart`; `PostToolUse` for Edit, Write, MultiEdit and NotebookEdit; `Stop` |
| Codex | `.codex/hooks.json` | `SessionStart`; `PostToolUse` for apply_patch; `Stop` |

`--host claude` or `--host codex` installs for one host. Installing again is
safe: it replaces the workflow hook and keeps every other setting and hook.
Status reports what each file lists; it cannot see whether a host loaded the
hooks. A hook written from another environment, such as outside WSL, counts as
installed, and status names the difference, because installing from here
rewrites it and Codex then asks for trust again.

- Claude Code reads project hooks when a session starts.
- Codex runs project hooks only after you trust them. Open `/hooks` in Codex,
  trust the ArcLint hooks, and start a new session. The command is identical
  in every checkout, so the worktrees of one repository share that trust.

The hooks run the `arclint` on the host's `PATH`.

### WSL projects opened from Windows

When you install from inside WSL, the hooks also work for the Windows desktop
apps that open the project through `\\wsl.localhost`. Codex receives a
`commandWindows` that runs
`wsl.exe -d <distribution> -- bash -lc "exec arclint agents workflow event"`.
Claude Code runs hook commands through Git Bash on Windows, so its command
runs `arclint` when the shell finds it and otherwise the same `wsl.exe`
command. That command needs Git Bash; Claude Code falls back to PowerShell
only when Git Bash is missing. The hook maps `\\wsl.localhost\<distribution>\...` and
`\\wsl$\<distribution>\...` paths to paths inside the distribution.

## What the agent receives

| When | Guidance |
|---|---|
| A session starts | The workflow order above. |
| A file changes before context showed every Zone that owns it | Run `arclint context <path>`. Each path is named once per session. Context shows the Zones that own the paths it names, the Zones named with `--zone`, or every Zone when it names neither. Context for a directory shows the Zones that own the directory, not narrower Zones nested inside it. A file no Zone owns needs no context. |
| A file other than the domain recording changes before the recording changed in the session | Record a new or changed meaning first; otherwise continue. Given once per session. |
| A turn finishes with changes no `arclint check` has verified | Run `arclint check .` and the tests. Given again only when a file no reminder has named changes. The user sees it as a system message. |

Guidance during work arrives as hook additional context, which the agent reads
with the tool result. Domain decisions live in the recorded `WorkflowGuide`
domain service; the hook adapter only reports what happened.

## How the hooks observe work

- ArcLint reports its own work. After `arclint context`, a full
  `arclint check`, or `arclint domain define|remove|init` runs, it appends
  the activity to `.arclint/cache/workflow/activity.jsonl`. It does so however
  the command was started: from Bash or PowerShell, through `wsl.exe`, or
  inside a script or make target. The hooks resolve the Zones that own the
  named paths exactly as `arclint context` does. A failed command, `--help`,
  a check narrowed by `--only` or `--exclude`, and a domain command that left
  the recording unchanged record nothing. ArcLint records only where the hooks
  created that directory, so projects without the hooks, and CI, gain no
  files.
- The hosts report edits. The file paths of Edit, Write, MultiEdit and
  NotebookEdit, and the file headers of an apply_patch, are changes. A change
  to `domain.arclint.yaml` changes the domain recording.
- Each session keeps its progress in `.arclint/cache/workflow/`, keyed by the
  host's session id. A session starts reading the activity log where it ends
  when the session starts, so earlier work is not credited to it. Parallel tool
  calls of one session update its progress one at a time. Records untouched
  for 30 days are removed when a new session starts.
- If a session cannot keep its progress, for example because `.arclint` is not
  writable, it says so once when it starts and then stays silent.

## Limits

- Files a shell command changes, such as through `>`, `sed -i`, `cp` or a
  script, are not observed; only the hosts' edit tools are.
- While `rules.arclint.yaml` cannot be read, the hooks cannot tell which Zones
  own a file and give no context reminder.
- Everything that runs ArcLint in one checkout shares its activity log:
  context or a check run in your terminal, a make target or another session
  counts for every open session. Separate worktrees do not share it.
- Paths outside the project, `.git/` and `.arclint/cache/` are ignored. Under
  WSL, a Windows drive path such as `C:\Users` maps to `/mnt/c/Users`.
- The hooks judge order, not meaning. The
  [domain reviewer](../domain-reviewer/) questions meanings, enforcement,
  ownership and duplication.
