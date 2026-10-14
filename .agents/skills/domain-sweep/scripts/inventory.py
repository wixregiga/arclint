#!/usr/bin/env python3
"""List the lines of a change that may state a rule, one line each, as file:line.

Reads the repository through git only; writes nothing inside it.

  inventory.py [--repo DIR] [--base REF] [--scope added|touched|both]
               [--paths GLOB ...] [--pattern REGEX] [--test-pattern REGEX]
               [--format tsv|md] [--out FILE] [--files]

The change is the working tree compared with --base (committed and uncommitted
work together) plus untracked files that git does not ignore. A line counts as
"added" when the diff adds it and "existing" when it was already in a touched
file. --scope added emits only added lines; touched emits every matching line
of a touched file; both emits added and existing lines, labelled.

Rules are often written without a keyword ("the first of these files that
exists, else this one"), so prose is listed whole: every line of a Markdown
file, every Go line or block comment line and every line of a Go raw string
(CLI help text)
is a row, with or without a keyword. Go code lines are rows only when a
keyword or a rule-like test name matches. --keywords-only restores the keyword
filter for prose too.

Prose is listed in every scope: --scope touched and the existing rows of
--scope both include the prose already in a touched file, where a rule the
change moved or made enforceable may be written.

A file the change deletes is listed from base, its rows of kind deleted, in
every scope but added: a removed statement may need a removed entry.

--files lists deleted files too, so the sweep runs arclint context on the
zones that governed them.

Output rows are: kind, match, file:line, text. kind is added, existing or deleted; match
is prose or keyword. --files prints the touched files only, one per line, for
`arclint context`.
"""

import argparse
import fnmatch
import os
import re
import subprocess
import sys

DEFAULT_PATHS = [
    "docs/site/content/docs/*.md",
    "README.md",
    "CONTRIBUTING.md",
    "AGENTS.md",
    "*.go",
]

DEFAULT_PATTERN = (
    r"\b(must|never|always|only|refuse[sd]?|refusal|reject(s|ed|ion)?|"
    r"forbid(s|den)?|cannot|can not|exactly one|at most|at least|no longer|"
    r"usage error|is an error|left untouched|unchanged|ignored)\b"
)

DEFAULT_TEST_PATTERN = (
    r"^func Test\w*(Refus|Reject|Never|Ignor|Only|Keep|Preserv|Untouch|"
    r"Unchanged|Distinct|Inside|Outside|Error|Fail|Forbid|Without|Says|Silent)\w*\("
)

HUNK = re.compile(r"^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@")


def git(repo, *args):
    # core.quotePath=false keeps non-ASCII names readable; names Git still
    # quotes (a quote, a backslash or a control character) are decoded by
    # git_path.
    result = subprocess.run(
        ["git", "-C", repo, "-c", "core.quotePath=false", *args],
        capture_output=True,
        text=True,
        errors="replace",
    )
    if result.returncode != 0:
        sys.stderr.write(result.stderr)
        sys.exit(1)
    return result.stdout


GIT_ESCAPES = {"a": 7, "b": 8, "t": 9, "n": 10, "v": 11, "f": 12, "r": 13, "\\": 92, '"': 34}


def git_path(spelled):
    """Decode a path as Git prints it in a diff header. A name Git quoted,
    such as "docs/\\303\\251.md" or one with a literal "é" beside an
    escaped line break, becomes the file's real name: escapes and octal
    bytes are read as bytes, literal characters as UTF-8, and the whole
    is decoded once."""
    if not (len(spelled) >= 2 and spelled.startswith('"') and spelled.endswith('"')):
        return spelled
    body = spelled[1:-1]
    out = bytearray()
    i = 0
    while i < len(body):
        ch = body[i]
        if ch != "\\" or i + 1 >= len(body):
            out += ch.encode("utf-8")
            i += 1
            continue
        nxt = body[i + 1]
        octal = body[i + 1:i + 4]
        if len(octal) == 3 and all(c in "01234567" for c in octal):
            out.append(int(octal, 8))
            i += 4
        elif nxt in GIT_ESCAPES:
            out.append(GIT_ESCAPES[nxt])
            i += 2
        else:
            out += ch.encode("utf-8")
            i += 1
    return out.decode("utf-8", "surrogateescape")


def matches_paths(path, globs):
    return any(fnmatch.fnmatch(path, g) for g in globs)


def added_lines_by_file(repo, base):
    """Map new-file path to the set of line numbers the diff adds, and list
    the files the change deletes, by their path in base."""
    out = git(repo, "diff", "-U0", "--no-color", "--no-ext-diff", "-M", base)
    added = {}
    deleted = []
    current = None
    old_path = None
    new_line = 0
    # "---" and "+++" name the files only in a file header, before the
    # first hunk; inside a hunk they are content lines that begin "--" or
    # "++" with the diff's own sign in front.
    in_header = False
    for raw in out.splitlines():
        if raw.startswith("diff --git"):
            in_header, current, old_path = True, None, None
            continue
        if raw.startswith("@@"):
            in_header = False
        if in_header and raw.startswith("--- "):
            source = git_path(raw[4:].strip())
            old_path = None if source == "/dev/null" else (source[2:] if source.startswith("a/") else source)
            continue
        if in_header and raw.startswith("+++ "):
            target = git_path(raw[4:].strip())
            if target == "/dev/null":
                current = None
                if old_path is not None:
                    deleted.append(old_path)
            else:
                current = target[2:] if target.startswith("b/") else target
                added.setdefault(current, set())
            continue
        if in_header:
            continue
        if current is None:
            continue
        hunk = HUNK.match(raw)
        if hunk:
            new_line = int(hunk.group(1))
            continue
        if raw.startswith("+"):
            added[current].add(new_line)
            new_line += 1
        elif raw.startswith(" "):
            new_line += 1
        # "-" and "\ No newline" lines do not advance the new file.
    return added, deleted


def untracked_files(repo):
    out = git(repo, "ls-files", "-z", "--others", "--exclude-standard")
    return [name for name in out.split("\0") if name]


def read_lines(repo, path):
    full = os.path.join(repo, path)
    try:
        with open(full, "r", encoding="utf-8", errors="replace") as handle:
            return handle.read().splitlines()
    except (OSError, IsADirectoryError):
        return None


def prose_lines(path, lines):
    """Return the line numbers of path that are prose: every non-blank line of
    a Markdown file; in a Go file, line comments, block comments, and the
    lines of raw string literals, where the CLI help text lives."""
    if path.endswith(".md"):
        return {n for n, text in enumerate(lines, start=1) if text.strip()}
    if not path.endswith(".go"):
        return set()
    prose = set()
    state = "code"  # code, raw (inside a raw string) or block (inside /* */)
    for number, text in enumerate(lines, start=1):
        touched = state != "code"
        i = 0
        while i < len(text):
            pair = text[i:i + 2]
            if state == "code":
                if pair == "//":
                    touched = True
                    break
                if pair == "/*":
                    state, touched = "block", True
                    i += 2
                    continue
                if text[i] == "`":
                    state, touched = "raw", True
                elif text[i] in "\"'":
                    # Skip an interpreted string or a rune literal, such as
                    # '`' or '\'', so its contents are not read as comment or
                    # raw string markers.
                    quote = text[i]
                    i += 1
                    while i < len(text) and text[i] != quote:
                        i += 2 if text[i] == "\\" else 1
            elif state == "block":
                if pair == "*/":
                    state = "code"
                    i += 2
                    continue
            elif state == "raw" and text[i] == "`":
                state = "code"
            i += 1
        if touched and text.strip():
            prose.add(number)
    return prose


def collect(repo, base, scope, globs, pattern, test_pattern, keywords_only):
    keyword = re.compile(pattern, re.IGNORECASE)
    test_name = re.compile(test_pattern) if test_pattern else None
    added, deleted = added_lines_by_file(repo, base)
    for path in untracked_files(repo):
        lines = read_lines(repo, path)
        if lines is None:
            continue
        added[path] = set(range(1, len(lines) + 1))

    rows = []
    files = []
    for path in sorted(added):
        if not matches_paths(path, globs):
            continue
        lines = read_lines(repo, path)
        if lines is None:
            continue
        files.append(path)
        prose = set() if keywords_only else prose_lines(path, lines)
        for number, text in enumerate(lines, start=1):
            is_added = number in added[path]
            if scope == "added" and not is_added:
                continue
            stripped = text.strip()
            if keyword.search(stripped) or (test_name and test_name.search(stripped)):
                match = "keyword"
            elif number in prose:
                match = "prose"
            else:
                continue
            kind = "added" if is_added else "existing"
            rows.append((kind, match, path, number, stripped))
    # A deleted file may have stated a rule the change now removes; its
    # lines come from base, at their line numbers there.
    for path in sorted(deleted):
        if matches_paths(path, globs):
            files.append(path)
    if scope != "added":
        for path in sorted(deleted):
            if not matches_paths(path, globs):
                continue
            lines = git(repo, "show", f"{base}:{path}").splitlines()
            prose = set() if keywords_only else prose_lines(path, lines)
            for number, text in enumerate(lines, start=1):
                stripped = text.strip()
                if keyword.search(stripped) or (test_name and test_name.search(stripped)):
                    match = "keyword"
                elif number in prose:
                    match = "prose"
                else:
                    continue
                rows.append(("deleted", match, path, number, stripped))
    return files, rows


def shown(path):
    """Spell a path for a line-based report: a backslash and every control
    character are escaped, so a name with a line break or a tab stays one
    field on one line. --files -z prints names unchanged instead."""
    out = []
    for ch in path:
        if ch == "\\":
            out.append("\\\\")
        elif ch == "\n":
            out.append("\\n")
        elif ch == "\t":
            out.append("\\t")
        elif ord(ch) < 0x20 or ord(ch) == 0x7F:
            out.append(f"\\x{ord(ch):02x}")
        else:
            out.append(ch)
    return "".join(out)


def render(rows, fmt):
    if fmt == "md":
        out = ["| kind | match | source | text |", "|---|---|---|---|"]
        for kind, match, path, number, text in rows:
            cell = text.replace("|", "\\|")
            out.append(f"| {kind} | {match} | {shown(path)}:{number} | {cell} |")
        return "\n".join(out) + "\n"
    return "".join(f"{kind}\t{match}\t{shown(path)}:{number}\t{text}\n" for kind, match, path, number, text in rows)


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--repo", default=".")
    parser.add_argument("--base", default="main")
    parser.add_argument("--scope", choices=["added", "touched", "both"], default="both")
    parser.add_argument("--paths", nargs="*", default=DEFAULT_PATHS)
    parser.add_argument("--pattern", default=DEFAULT_PATTERN)
    parser.add_argument("--test-pattern", default=DEFAULT_TEST_PATTERN)
    parser.add_argument("--format", choices=["tsv", "md"], default="tsv")
    parser.add_argument("--out")
    parser.add_argument("--files", action="store_true", help="print touched files only")
    parser.add_argument("-z", action="store_true", help="with --files, end each name with NUL and print it unchanged")
    parser.add_argument("--keywords-only", action="store_true", help="filter prose by keyword as well")
    args = parser.parse_args()

    repo = os.path.abspath(args.repo)
    git(repo, "rev-parse", "--verify", "--quiet", args.base + "^{commit}")
    files, rows = collect(repo, args.base, args.scope, args.paths, args.pattern, args.test_pattern, args.keywords_only)

    if args.files:
        if args.z:
            sys.stdout.write("".join(f"{path}\0" for path in files))
        else:
            sys.stdout.write("".join(f"{shown(path)}\n" for path in files))
        return

    text = render(rows, args.format)
    if args.out:
        with open(args.out, "w", encoding="utf-8") as handle:
            handle.write(text)
    else:
        sys.stdout.write(text)
    counts = {kind: sum(1 for row in rows if row[0] == kind) for kind in ("added", "existing", "deleted")}
    prose_count = sum(1 for row in rows if row[1] == "prose")
    sys.stderr.write(
        f"inventory: {len(files)} files touched, {len(rows)} lines "
        f"({counts['added']} added, {counts['existing']} existing, {counts['deleted']} deleted; "
        f"{prose_count} prose, {len(rows) - prose_count} keyword)\n"
    )


if __name__ == "__main__":
    main()
