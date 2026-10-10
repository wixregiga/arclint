---
title: Agent hooks
weight: 8
description: Advise Claude Code and Codex when observed work skips a step of the domain workflow.
---

Coding agents often change code before they look up the architecture or
record the meaning a change introduces. ArcLint's workflow hooks watch what
the agent does and advise when it skips a step of the project's workflow:

1. Run `arclint context <paths>` on the files you will read or change,
   before opening them.
2. Before editing, state what the change does, who owns its decisions,
   what the caller uses and which existing behavior must remain intact.
3. If the change introduces or changes a meaning, record it in
   `domain.arclint.yaml` before writing code, using the domain-librarian
   skill. If it does not, say so.
4. Implement one complete behavior path through the zones
   `arclint context` reported, keeping existing behavior intact.
5. Verify the changed behavior, then run `arclint check .` and the
   project's tests. Fix the findings in the code you changed, including
   baseline findings there; never weaken rules, baselines or exclusions to
   clear a finding.

The generated `AGENTS.md` block opens with these steps in full, and the
hooks repeat them when a session starts; both come from one source in
ArcLint.

The hooks advise; they never block a tool call. Each reminder is given once,
so repeating an action does not repeat the reminder. Any project using
ArcLint can install them, and ArcLint uses them for its own development.

## Install

```sh
arclint agents hooks install
arclint agents hooks status
```

Installation writes one hook command into each host's user configuration, so
the hooks run in every project and worktree the host opens:

| Host | File | Events |
|---|---|---|
| Claude Code | `~/.claude/settings.json`, or under `CLAUDE_CONFIG_DIR` | `SessionStart`; `PostToolUse` for Edit, Write, MultiEdit and NotebookEdit; `Stop` |
| Codex | `~/.codex/hooks.json`, or under `CODEX_HOME` | `SessionStart`; `PostToolUse` for apply_patch; `Stop` |

The command runs the `arclint` that installed it, by its absolute path, with
`agents hooks event`. It does not depend on the hosts' `PATH` or on shell
startup files, and an upgrade that replaces the binary at that path takes
effect at the next event. The path may hold only letters, digits and `._/+-`,
so every host shell runs it unquoted; install refuses any other path.

In a project without `rules.arclint.yaml` the hooks answer nothing and write
nothing, so one user install serves every repository.

`--project` writes the project's files instead, for that project only:
`.claude/settings.local.json` and `.codex/hooks.json`. Neither should be
committed with the hooks in it, because the command names a path on your
machine. A project install is refused while a user configuration already lists
the hooks. A user install removes them from `.claude/settings.local.json`,
which belongs to one user; it leaves `.codex/hooks.json`, which a repository
may share, and status reports that duplicate. When two files list the hooks,
the host runs each event twice and ArcLint answers the first.

`--host claude` or `--host codex` installs for one host. Installing again is
safe: it replaces the workflow hook and keeps every other setting and hook. A
hook counts as the workflow hook only when its command is one install writes,
for any binary path and distribution; a hook that merely mentions the same
words is kept. A
user configuration linked from elsewhere stays linked. Install prints the
workflow steps, so the session that ran it can follow them before the hooks
reach it.

`arclint agents hooks status` lists every file that can hold the hooks for
the project, the binary install would pin, and what is wrong in each file: a
missing or changed event, a binary that no longer exists or is not executable,
or the same hooks in two files. It also shows
when a hook event last reached the project. The files show what a host is
configured to run; the last event shows that a host ran the hooks here.

- Claude Code runs the hooks in sessions that start after installation.
- Codex runs hooks only after you trust them. Open `/hooks` in Codex, trust the
  ArcLint hooks, and start a new session.

### WSL projects opened from Windows

When you install from inside WSL, the install also writes the Windows profile's
`.claude\settings.json` and `.codex\hooks.json`, which the Windows desktop apps
read when they open the project through `\\wsl.localhost`. ArcLint finds the
profile, and `CLAUDE_CONFIG_DIR` or `CODEX_HOME` when Windows sets them,
through `cmd.exe`. Those files run
`wsl.exe -d <distribution> -e <path> agents hooks event`. Claude Code runs
hook commands through Git Bash on Windows, which rewrites arguments that look
like paths, so its command sets `MSYS_NO_PATHCONV=1`. That command needs Git
Bash; Claude Code falls back to PowerShell only when Git Bash is missing. Each
host app reads only its own side's file, so a session receives each event once.
A `--project` install from WSL writes a command that runs the binary when it
exists on the reader's side and `wsl.exe` otherwise. The hook maps
`\\wsl.localhost\<distribution>\...` and `\\wsl$\<distribution>\...` paths to
paths inside the distribution.

## What the agent receives

| When | Guidance |
|---|---|
| A session starts | The workflow order above. When the project's `AGENTS.md` block states a different workflow, a note names the `arclint` the hooks run: that binary and the one that generated the block are different releases. |
| A file changes before context showed every Zone that owns it | Run `arclint context <path>`. Each path is named once per session. Context shows the Zones that own the paths it names, the Zones named with `--zone`, or every Zone when it names neither. Context for a directory shows the Zones that own the directory, not narrower Zones nested inside it. A file no Zone owns needs no context. |
| A file other than the domain recording changes before the recording changed in the session | Record a new or changed meaning first; otherwise continue. Given once per session. |
| A turn finishes with changes no `arclint check` has verified | Run `arclint check .` and the tests. Given again only when a file no reminder has named changes. The user sees it as a system message. |

Guidance during work arrives as hook additional context, which the agent reads
with the tool result. When a host delivers the same event twice in a session,
as it does when two configuration files list the hooks, only the first
delivery is answered. Domain decisions live in the recorded `WorkflowGuide`
domain service; the hook adapter only reports what happened.

## How the hooks observe work

- ArcLint reports its own work. After `arclint context`, a full
  `arclint check`, or `arclint domain define|remove|init` runs, it appends
  the activity to `.arclint/cache/hooks/activity.jsonl`. It does so however
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
- Each session keeps its progress in `.arclint/cache/hooks/`, keyed by the
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
- The hooks judge order, not meaning.
