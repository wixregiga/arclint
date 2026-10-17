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
   what the caller uses, which existing behavior must remain intact and
   which properties the change newly promises, each with the test that
   proves it. For a component that reads and writes a format, the promises
   are: one encoder for fresh and edited output; print, then parse, then
   print gives the same bytes; an edit changes only its own lines; a
   repeated edit changes nothing; the author's optional tokens survive.
3. If the change introduces or changes a meaning, record it in
   the project's domain recording through `arclint domain define` before
   writing code, using the domain-librarian skill. If it does not, say so.
4. Implement one complete behavior path through the zones
   `arclint context` reported, keeping existing behavior intact.
5. Verify the changed behavior, then run `arclint check .` and the
   project's tests. Fix the findings in the code you changed, including
   baseline findings there; never weaken rules, baselines or exclusions to
   clear a finding. Copying code, or moving it only to clear a finding, is
   not a fix: when a rule refuses what the change needs, report the finding
   and propose the zone change that would allow it.

The generated `AGENTS.md` block opens with these steps in full, and the
hooks repeat them when a session starts; both come from one source in
ArcLint. The instructions do not name a domain file or format: agents learn
the recording path from `arclint context` and `arclint domain`. The librarian
records through `arclint domain define` and `arclint domain remove`, so ArcLint
validates each write and writes the format. Switching formats leaves the
instructions current.

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
| Claude Code | `~/.claude/settings.json`, or under `CLAUDE_CONFIG_DIR` | `SessionStart`; `UserPromptSubmit`; `PostToolUse` for Edit, Write, MultiEdit and NotebookEdit; `Stop` |
| Codex | `~/.codex/hooks.json`, or under `CODEX_HOME` | `SessionStart`; `UserPromptSubmit`; `PostToolUse` for apply_patch; `Stop` |

No hook runs on shell commands. Files a shell command changes are found at
the events above, as described under [how the hooks observe work](#how-the-hooks-observe-work).

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

Each file is `complete` when it lists every hook, `partial` when it lists some,
`absent` when it lists none, and `unknown` when it cannot be read. A host runs
every hook a partial file lists. A file written by an older install, for
example, lacks `UserPromptSubmit` and still runs its other hooks; status names
the hooks it lists and the ones it lacks. It says a hook still runs only when
the hook is the one install writes there, because a changed hook may name a
binary that is gone or a matcher that selects nothing. While a Claude Code
file sets `disableAllHooks`, status names that file and says no hook runs for
the host that reads it: the distribution's Claude Code reads its own user
file, and the Windows apps read the Windows profile's. In `--format json`, `installed`
carries the state, and `listed` and `missing` name the events.

Until an event arrives, status says why none may have: no file lists the
hooks, or none lists all of them while the ones listed still run; the
directory holds no `rules.arclint.yaml`; or it does, but a session
started before it existed, as in a worktree made from an older commit, and
reports here only at its next prompt, edit or turn end. It also notes that a
session reports to the project holding its working directory, and that Claude
Code runs the hooks only in sessions started after installation.

Inside WSL, status reads the Windows profile through `cmd.exe`. When
`cmd.exe` cannot run, the Windows files are listed with that problem, and the
distribution's files are reported as usual. Install refuses in that case,
because it cannot write the Windows files it promises.

- Claude Code runs the hooks in sessions that start after installation.
- Codex runs hooks only after you trust them. Open `/hooks` in Codex, trust the
  ArcLint hooks, and start a new session. Codex trusts each hook definition by
  its content, so a hook that install adds or changes, such as
  `UserPromptSubmit`, needs trusting again.

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
| A file that bears domain meaning changes before the recording changed in the session | Record a new or changed meaning first; otherwise continue. Given once per session. A file bears domain meaning when it is not a test and lies in a Zone that holds a bounded context's code, the Zone named for the context. While a recorded context has no Zone named for it, or no context is recorded yet, every file other than a test qualifies, as `arclint check` reads the model's code. Tests and files outside the model's code receive no reminder and do not use it up. |
| A turn finishes with changes no `arclint check` has verified | Run `arclint check .` and the tests. Given again only when a file no reminder has named changes. The user sees it as a system message. |

Guidance during work arrives as hook additional context, which the agent reads
with the tool result or with the user's prompt. When a host delivers the same event twice in a session,
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
  a check narrowed by `--only` or `--exclude`, a check that left a Rule not
  evaluated (exit `3`), and a domain command that left the recording
  unchanged record nothing. ArcLint records only where the hooks
  created that directory, so projects without the hooks, and CI, gain no
  files.
- The hosts report edits. The file paths of Edit, Write, MultiEdit and
  NotebookEdit, and the file headers of an apply_patch, are changes. A change
  to the project's domain file (the file `rules.arclint.yaml` names under
  `domain.file`, else the first of `domain.arclint.yaml`, `domain.arclint.yml`
  and `domain.arclint.cml` that exists) changes the domain recording.
- The checkout shows the rest. At each event, the hooks compare the files
  `git status` shows changed (modified, added, deleted, or untracked and not
  ignored) with the snapshot the session took at its last event. A file that
  appears, changes again, disappears, or is restored counts as changed, so
  edits through `sed -i`, `>`, `cp`, a Python heredoc or a script get the same
  advice as an edit tool's, once, at the next event. A commit changes no file.
  Each change takes its place by the time the file system stamped on it
  among the activities ArcLint recorded, so a shell edit after
  `arclint context` needs no context, and one before `arclint check` is
  verified by it. ArcLint reads its own times from the same file system
  clock, by writing `.arclint/cache/hooks/clock`, because file times run
  milliseconds behind the machine's clock. A change in the same clock tick as
  an ArcLint command, or as the start or end of a time between turns, counts
  as the agent's and as made after the command. A turn that changed nothing
  prints nothing.
- Files that change between turns are not the agent's work. The session sees
  the checkout afresh when it starts or resumes and at the user's next prompt
  after a finished turn. A turn that never finished, such as one the user
  interrupted, is compared at the next prompt or when the session resumes, and
  the agent receives the advice with that prompt or with the workflow order.
- Each session keeps its progress in `.arclint/cache/hooks/`, keyed by the
  host's session id. A session starts reading the activity log where it ends
  when the session starts, so earlier work is not credited to it. Parallel tool
  calls of one session update its progress one at a time. Records untouched
  for 30 days are removed when a new session starts.
- If a session cannot keep its progress, for example because `.arclint` is not
  writable, it says so once when it starts and then stays silent.

## Cost

Each event runs `git status` once, without git's optional index lock, so it
never makes a git command the agent runs at the same moment fail. That run is
the comparison's cost. Measured per edit event on one 16-core Linux machine
under load, with 20 modified files, as the median of three rounds of 15
events each, wall time:

| Repository | Tracked files | Before | After |
|---|---|---|---|
| ArcLint | 1196 | 60 ms | 70 ms |
| kubernetes | 31441 | 40 ms | 370 ms |
| TypeScript | 67916 | 30 ms | 320 ms |
| linux | 96080 | 30 ms | 450 ms |

The time grows with the number of files git examines. Right after a clone or
checkout, git may read every file again until a git command of your own
refreshes the index.

## Limits

- Shell changes are observed only in a git work tree, and only for files git
  does not ignore. Submodules are not compared. Without git, only the hosts'
  edit tools report changes.
- Anything else that changes the checkout during a turn, such as your editor or
  another session in the same checkout, counts as the agent's change.
- Order comes from the file system's clock, read where the project lives. A
  file inside the project that lies on another file system, such as a
  mounted directory, is stamped by that file system's clock. What a parallel
  tool call does while an event samples the checkout waits for the next
  event.
- When `git status` fails at one event, the hooks wait for the next one.
  When it fails at two events in a row, ArcLint's own activities are read
  without the checkout, so a repository git cannot read never holds them
  back; a shell change made before such an outage may then be ordered after
  them.
- A removed file takes the time its directory last changed, which is later
  than the removal when another file of that directory changed after it.
- On Windows, which records no change time, a copy that keeps a file's size,
  modification time and mode, as `cp -p` does, is not seen.
- While `rules.arclint.yaml` cannot be read, the hooks cannot tell which Zones
  own a file and give no context reminder.
- Everything that runs ArcLint in one checkout shares its activity log:
  context or a check run in your terminal, a make target or another session
  counts for every open session. Separate worktrees do not share it.
- Paths outside the project, `.git/` and `.arclint/cache/` are ignored. Under
  WSL, a Windows drive path such as `C:\Users` maps to `/mnt/c/Users`.
- The hooks judge order, not meaning.
