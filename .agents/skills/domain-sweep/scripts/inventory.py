#!/usr/bin/env python3
"""List every line of a change that a reader of its rules must read, as file:line.

Reads the repository through git only; writes nothing inside it.

  inventory.py [--repo DIR] [--base REF] [--head REF] [--scope changed|touched]
               [--paths GLOB ...] [--exclude GLOB ...] [--recording PATH ...]
               [--hint REGEX] [--format tsv|md] [--out FILE] [--files]

The change runs from the merge base of --base and the head to the head. With
--head the head is that commit; without it the head is the working tree,
committed and uncommitted work together, plus untracked files that git does
not ignore.

No keyword decides whether a line is read. A rule is often written without
one ("the first of these files that exists, else this one"), so every line of
these sources is a row, whole:

  doc        every non-blank line of a Markdown file
  recording  every non-blank line of a domain recording: a file named
             domain.arclint.yaml, domain.arclint.yml or domain.arclint.cml,
             or one --recording names
  comment    a comment line in Go, TypeScript or Python
  text       a line of a string literal that spans lines (Go raw strings,
             where CLI help text lives; TypeScript template literals; Python
             triple-quoted strings and docstrings), or a line holding a
             one-line literal with two words in it, such as an error message
  test       a test declaration: Go func Test..., Python def test_... and
             class Test..., a TypeScript it(, test( or describe( call

Other lines of code are not rows: code is enforcement evidence, not a
statement. In a test file (Go *_test.go, TypeScript *.test.ts and *.spec.ts,
Python test_*.py and *_test.py) literals are fixtures: its comments and test
declarations are rows, its text is not. JSX text in .tsx and .jsx files is
not recognised as text; read those files' changed hunks whole.

--exclude drops files that state nothing of their own: by default the
.claude/skills copies of .agents/skills, everything under a testdata
directory (fixtures and vendored material, which the Go tool ignores too),
and the root domain.arclint.cml, which make generate exports from
domain.arclint.yaml. A glob that starts with / matches from the root.

A file that declares itself generated ("Code generated ... DO NOT EDIT" in a
comment before its content) repeats its generator's text and has no rows;
--files still lists it, and the summary names it.

Rows are: kind, source, hint, file:line, text. A file name that holds a
control character or a backslash, or starts with a double quote, is printed
in double quotes with C escapes, as Git prints it.
  kind    added (the change adds the line), deleted (the change removes it;
          file:line is the line in the merge base) or existing (an unchanged
          line of a file the change touches)
  source  doc, recording, comment, text or test, as above
  hint    the first word of --hint the line contains, or -

--scope changed emits added and deleted rows; touched (the default) also
emits the existing rows of touched files, where a rule the change moved or
made enforceable may be written.

Keywords only order the reading: rows come by kind (added, deleted,
existing), then by file, files with more hinted rows first, then by line, so
each file's prose stays in reading order.

--files prints the touched files only, deleted ones included, one per line,
for `arclint context`.
"""

import argparse
import fnmatch
import os
import re
import subprocess
import sys

DEFAULT_PATHS = ["*.md", "*.go", "*.ts", "*.tsx", "*.mts", "*.cts", "*.py"]

# Files that state nothing of their own: byte copies of .agents/skills,
# fixtures and vendored material, and the CML export of the root's
# recording.
DEFAULT_EXCLUDE = [
    ".claude/skills/*",
    "*/.claude/skills/*",
    "testdata/*",
    "*/testdata/*",
    "/domain.arclint.cml",
]

RECORDING_NAMES = ["domain.arclint.yaml", "domain.arclint.yml", "domain.arclint.cml"]

DEFAULT_HINT = (
    r"\b(must not|must|never|always|only|refuse[sd]?|refusal|reject(?:s|ed|ion)?|"
    r"forbid(?:s|den)?|cannot|can not|exactly one|at most|at least|no longer|"
    r"usage error|is an error|untouched|unchanged|ignored|required|requires|"
    r"guarantee[sd]?|ensures?|every|otherwise|unless|first of|default)\b"
)

HUNK = re.compile(r"^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@")

GENERATED = re.compile(r"Code generated .* DO NOT EDIT")

# Two words in a one-line literal: a message or a sentence, not an import
# path, a key, a format verb or a struct tag.
WORDS = re.compile(r"[A-Za-z]{2,}[ \t]+[A-Za-z]{2,}")

# Test declarations, matched against a line's code alone: every literal on
# the line reads "", so a declaration written inside a fixture string is
# not one. A TypeScript call whose name is on the next line ends in "(".
GO_TEST = re.compile(r"^\s*func\s+Test\w*\s*\(")
PY_TEST = re.compile(r"^\s*(?:async\s+)?def\s+test\w*\s*\(|^\s*class\s+Test\w*\b")
TS_TEST = re.compile(r"(?:^|[^\w.$])(?:it|test|describe)(?:\.\w+)*\s*\(\s*(\"\"|$)")
LITERAL = '""'

TS_SUFFIXES = (".ts", ".tsx", ".mts", ".cts", ".js", ".jsx", ".mjs", ".cjs")

KIND_ORDER = {"added": 0, "deleted": 1, "existing": 2}


def git(repo, *args):
    result = subprocess.run(["git", "-C", repo, *args], capture_output=True)
    if result.returncode != 0:
        sys.stderr.write(result.stderr.decode("utf-8", "replace"))
        sys.exit(1)
    return result.stdout


def git_text(repo, *args):
    return git(repo, *args).decode("utf-8", "replace")


def git_paths(repo, *args):
    """The NUL-separated paths a -z command prints, decoded as the file
    system spells them, so a name that is not UTF-8 still opens."""
    return [os.fsdecode(name) for name in git(repo, *args).split(b"\0") if name]


def matches(path, globs):
    """A glob without a slash matches the file's name in any directory; a
    glob with one matches the whole path, and one that starts with a slash
    matches from the root."""
    name = path.rsplit("/", 1)[-1]
    for glob in globs:
        if glob.startswith("/"):
            hit = fnmatch.fnmatchcase(path, glob[1:])
        else:
            hit = fnmatch.fnmatchcase(path if "/" in glob else name, glob)
        if hit:
            return True
    return False


TEST_FILE = re.compile(r"(_test\.go|\.(test|spec)\.[cm]?[jt]sx?|_test\.py)$|(^|/)test_[^/]*\.py$")


def touched_entries(repo, base, head):
    """The files the change touches, as (old path, new path): old is None
    for an added file, new is None for a deleted one, and a rename carries
    both. Paths come from -z output, never from a diff header."""
    target = [head] if head else []
    fields = git_paths(repo, "diff", "-z", "--name-status", "--no-relative", "-M", base, *target)
    entries = []
    i = 0
    while i < len(fields):
        status = fields[i]
        if status[0] in "RC":
            entries.append((fields[i + 1] if status[0] == "R" else None, fields[i + 2]))
            i += 3
            continue
        path = fields[i + 1]
        entries.append((path if status[0] != "A" else None, path if status[0] != "D" else None))
        i += 2
    if not head:
        for path in git_paths(repo, "ls-files", "-z", "--others", "--exclude-standard"):
            entries.append((None, path))
    return entries


def changed_lines(repo, base, head, old, new):
    """The line numbers the change adds to new, in the head, and removes
    from old, in the merge base. Settings that would move hunk positions
    off the files' own lines are turned off: text conversion, external
    diff, joined hunks. Context lines a configuration still adds advance
    both sides."""
    target = [head] if head else []
    paths = [p for p in (old, new) if p is not None]
    out = git_text(
        repo, "--literal-pathspecs", "diff", "-U0", "--inter-hunk-context=0", "--no-color",
        "--no-ext-diff", "--no-textconv", "--no-relative", "-M", base, *target, "--", *dict.fromkeys(paths),
    )
    added, removed = set(), set()
    old_line = new_line = 0
    in_hunk = False
    for raw in out.split("\n"):
        if raw.startswith("diff --git"):
            in_hunk = False
            continue
        hunk = HUNK.match(raw)
        if hunk:
            old_line, new_line = int(hunk.group(1)), int(hunk.group(3))
            in_hunk = True
            continue
        if not in_hunk:
            continue
        if raw.startswith("+"):
            added.add(new_line)
            new_line += 1
        elif raw.startswith("-"):
            removed.add(old_line)
            old_line += 1
        elif not raw.startswith("\\"):
            # A context line; "\\ No newline at end of file" advances
            # neither side.
            old_line += 1
            new_line += 1
    return added, removed


def read_worktree(repo, path):
    full = os.path.join(repo, path)
    if os.path.islink(full) or not os.path.isfile(full):
        return None
    with open(full, "r", encoding="utf-8", errors="replace") as handle:
        return handle.read().splitlines()


def read_commit(repo, commit, path):
    result = subprocess.run(["git", "-C", repo, "cat-file", "blob", f"{commit}:{path}"], capture_output=True)
    if result.returncode != 0:
        return None
    return result.stdout.decode("utf-8", "replace").splitlines()


def is_generated(lines):
    """A file is generated when a comment before its content says "Code
    generated ... DO NOT EDIT"; front matter at the top of a Markdown page
    is skipped first."""
    i = 0
    if lines and lines[0].strip() in ("+++", "---"):
        fence = lines[0].strip()
        i = 1
        while i < len(lines) and lines[i].strip() != fence:
            i += 1
        i += 1
    for text in lines[i:]:
        stripped = text.strip()
        if not stripped:
            continue
        if stripped.startswith(("//", "#", "/*", "*", "<!--")):
            if GENERATED.search(stripped):
                return True
            continue
        return False
    return False


def scan_quoted(text, j, quote):
    """Scan a quoted literal's contents from text[j]. Return the index past
    it, its contents, and how it ends: "closed"; "continued", when the line
    ends in a backslash inside it and the literal goes on on the next line;
    or "open", when the line ends without closing it."""
    start = j
    while j < len(text):
        if text[j] == "\\":
            if j + 1 == len(text):
                return len(text), text[start:j], "continued"
            j += 2
            continue
        if text[j] == quote:
            return j + 1, text[start:j], "closed"
        j += 1
    return len(text), text[start:], "open"


def go_sources(lines):
    """Map line number to source for a Go file."""
    found = {}
    state = "code"  # code, raw (inside a raw string) or block (inside /* */)
    for number, text in enumerate(lines, start=1):
        kinds = set()
        code = []
        if state == "raw":
            kinds.add("text")
        elif state == "block":
            kinds.add("comment")
        i = 0
        while i < len(text):
            if state == "block":
                end = text.find("*/", i)
                if end < 0:
                    break
                state, i = "code", end + 2
                continue
            if state == "raw":
                end = text.find("`", i)
                if end < 0:
                    break
                state, i = "code", end + 1
                code.append(LITERAL)
                continue
            pair = text[i:i + 2]
            if pair == "//":
                kinds.add("comment")
                break
            if pair == "/*":
                state = "block"
                kinds.add("comment")
                i += 2
                continue
            char = text[i]
            if char == "`":
                end = text.find("`", i + 1)
                if end < 0:
                    state = "raw"
                    kinds.add("text")
                    break
                if WORDS.search(text[i + 1:end]):
                    kinds.add("text")
                code.append(LITERAL)
                i = end + 1
                continue
            if char in "\"'":
                i, content, _ = scan_quoted(text, i + 1, char)
                if char == '"' and WORDS.search(content):
                    kinds.add("text")
                code.append(LITERAL)
                continue
            code.append(char)
            i += 1
        if GO_TEST.search("".join(code)):
            kinds.add("test")
        if kinds and text.strip():
            found[number] = pick(kinds)
    return found


# A slash after one of these, or at the start of an expression, opens a
# regular expression literal; after anything else it divides.
REGEX_BEFORE = set("(,=:[!&|?{};+-*%<>~^")
REGEX_KEYWORDS = {"return", "typeof", "case", "do", "else", "in", "of", "new", "delete", "void", "throw", "yield", "await"}
# A parenthesis these open closes a condition, after which a slash starts a
# statement: if (x) /re/.test(s).
CONTROL_KEYWORDS = {"if", "while", "for", "with"}
def regex_allowed(text, i, last):
    if last is None:
        return True
    if last in REGEX_BEFORE:
        return True
    word = re.search(r"([A-Za-z_$][\w$]*)\s*$", text[:i])
    return bool(word) and word.group(1) in REGEX_KEYWORDS


def skip_regex(text, i):
    """Return the index past the regular expression literal at text[i]."""
    j = i + 1
    in_class = False
    while j < len(text):
        char = text[j]
        if char == "\\":
            j += 2
            continue
        if char == "[":
            in_class = True
        elif char == "]":
            in_class = False
        elif char == "/" and not in_class:
            j += 1
            while j < len(text) and (text[j].isalnum() or text[j] == "_"):
                j += 1
            return j
        j += 1
    return len(text)


def ts_sources(lines):
    """Map line number to source for a TypeScript or JavaScript file.
    Template literals may nest code in ${ }, which may nest literals; a
    stack keeps the brace depth of each open substitution."""
    found = {}
    state = "code"  # code, block, template, or string (a quoted literal continued past a line)
    quote = None  # the quote of a continued string
    stack = []  # brace depth in each enclosing ${ } substitution
    depth = 0
    parens = []  # for each open parenthesis, whether it opened a condition
    last = None  # last significant character of code, for regex detection
    pending_test = False  # a test call ended its line before its name
    for number, text in enumerate(lines, start=1):
        kinds = set()
        code = []
        if state in ("template", "string"):
            kinds.add("text")
        elif state == "block":
            kinds.add("comment")
        segment = []  # the template characters read on this line

        def end_segment():
            if WORDS.search("".join(segment)):
                kinds.add("text")
            segment.clear()

        i = 0
        while i < len(text):
            char = text[i]
            if state == "block":
                end = text.find("*/", i)
                if end < 0:
                    break
                state, i = "code", end + 2
                continue
            if state == "string":
                i, _, how = scan_quoted(text, i, quote)
                if how != "closed":
                    if how == "open":
                        state = "code"
                    break
                state, last = "code", quote
                code.append(LITERAL)
                continue
            if state == "template":
                if char == "\\":
                    segment.append(text[i:i + 2])
                    i += 2
                    continue
                if char == "`":
                    end_segment()
                    state, last = "code", "`"
                    i += 1
                    continue
                if text[i:i + 2] == "${":
                    end_segment()
                    stack.append(depth)
                    depth, state, last = 0, "code", "{"
                    i += 2
                    continue
                segment.append(char)
                i += 1
                continue
            pair = text[i:i + 2]
            if pair == "//":
                kinds.add("comment")
                break
            if pair == "/*":
                state = "block"
                kinds.add("comment")
                i += 2
                continue
            if char == "`":
                state = "template"
                code.append(LITERAL)
                i += 1
                continue
            if char in "\"'":
                i, content, how = scan_quoted(text, i + 1, char)
                if WORDS.search(content):
                    kinds.add("text")
                code.append(LITERAL)
                if how == "continued":
                    state, quote = "string", char
                    kinds.add("text")
                    break
                last = char
                continue
            if char == "/" and regex_allowed(text, i, last):
                i = skip_regex(text, i)
                code.append(LITERAL)
                last = "/"
                continue
            if char == "(":
                word = re.search(r"([A-Za-z_$][\w$]*)\s*$", text[:i])
                parens.append(bool(word) and word.group(1) in CONTROL_KEYWORDS)
            elif char == ")":
                closes_condition = parens.pop() if parens else False
                code.append(char)
                last = "(" if closes_condition else ")"
                i += 1
                continue
            elif char == "{":
                depth += 1
            elif char == "}":
                if depth == 0 and stack:
                    depth = stack.pop()
                    state = "template"
                    i += 1
                    continue
                depth -= 1
            code.append(char)
            if not char.isspace():
                last = char
            i += 1
        if state == "template":
            # The literal goes on past this line: every line it spans is text.
            kinds.add("text")
        end_segment()
        line_code = "".join(code)
        declared = TS_TEST.search(line_code)
        if declared or (pending_test and line_code.lstrip().startswith(LITERAL)):
            kinds.add("test")
        pending_test = bool(declared) and declared.group(1) == ""
        if kinds and text.strip():
            found[number] = pick(kinds)
    return found


PY_STRING = re.compile(r"([rRbBuUfF]{0,2})('\'\'|\"\"\"|'|\")")


def python_sources(lines):
    """Map line number to source for a Python file."""
    found = {}
    open_quote = None  # the quote of a string that goes on past a line
    for number, text in enumerate(lines, start=1):
        kinds = set()
        code = []
        i = 0
        if open_quote:
            kinds.add("text")
            end, how = find_close(text, 0, open_quote)
            if end < 0:
                if len(open_quote) == 1 and how != "continued":
                    open_quote = None
                if text.strip():
                    found[number] = "text"
                continue
            open_quote = None
            code.append(LITERAL)
            i = end
        while i < len(text):
            char = text[i]
            if char == "#":
                kinds.add("comment")
                break
            match = PY_STRING.match(text, i) if char in "'\"rRbBuUfF" else None
            if match:
                quote = match.group(2)
                body = match.end()
                end, how = find_close(text, body, quote)
                code.append(LITERAL)
                if end < 0:
                    if len(quote) == 3 or how == "continued":
                        open_quote = quote
                        kinds.add("text")
                    elif WORDS.search(text[body:]):
                        kinds.add("text")
                    break
                if WORDS.search(text[body:end - len(quote)]):
                    kinds.add("text")
                i = end
                continue
            if char.isalnum() or char == "_":
                j = i
                while j < len(text) and (text[j].isalnum() or text[j] == "_"):
                    j += 1
                code.append(text[i:j])
                i = j
                continue
            code.append(char)
            i += 1
        if PY_TEST.search("".join(code)):
            kinds.add("test")
        if kinds and text.strip():
            found[number] = pick(kinds)
    return found


def find_close(text, i, quote):
    """Return the index past the closing quote, or -1 when the line does
    not close the string, and "continued" when the line ends in a
    backslash inside it. A backslash escapes the next character, in raw
    strings too, where it stays in the value."""
    while i < len(text):
        if text[i] == "\\":
            if i + 1 == len(text):
                return -1, "continued"
            i += 2
            continue
        if text.startswith(quote, i):
            return i + len(quote), "closed"
        i += 1
    return -1, "open"


def pick(kinds):
    """One source per line: a test declaration first, then a comment, then
    text."""
    for kind in ("test", "comment", "text"):
        if kind in kinds:
            return kind
    raise ValueError(kinds)


def is_recording(path, recordings):
    return path in recordings or path.rsplit("/", 1)[-1] in RECORDING_NAMES


def line_sources(path, lines, recordings):
    """Map each row line number of path to its source."""
    if is_recording(path, recordings):
        return {n: "recording" for n, text in enumerate(lines, start=1) if text.strip()}
    if path.endswith(".md"):
        return {n: "doc" for n, text in enumerate(lines, start=1) if text.strip()}
    if path.endswith(".go"):
        found = go_sources(lines)
    elif path.endswith(TS_SUFFIXES):
        found = ts_sources(lines)
    elif path.endswith(".py"):
        found = python_sources(lines)
    else:
        return {}
    if TEST_FILE.search(path):
        return {n: source for n, source in found.items() if source != "text"}
    return found


def find_merge_base(repo, base, head):
    result = subprocess.run(["git", "-C", repo, "merge-base", base, head], capture_output=True, text=True, errors="replace")
    if result.returncode != 0:
        sys.stderr.write(f"inventory: {base} and {head} have no merge base\n{result.stderr}")
        sys.exit(1)
    return result.stdout.strip()


def collect(repo, base, head, scope, globs, exclude, recordings, hint):
    hint_word = re.compile(hint, re.IGNORECASE)
    merge_base = find_merge_base(repo, base, head or "HEAD")
    untracked = set() if head else set(git_paths(repo, "ls-files", "-z", "--others", "--exclude-standard"))

    def wanted(path):
        return path is not None and (matches(path, globs) or is_recording(path, recordings)) and not matches(path, exclude)

    def read_head(path):
        return read_commit(repo, head, path) if head else read_worktree(repo, path)

    files = set()
    generated = []
    rows = []

    def emit(kind, path, lines, chosen):
        if is_generated(lines):
            if path not in generated:
                generated.append(path)
            return
        sources = line_sources(path, lines, recordings)
        for number in sorted(sources):
            if number not in chosen:
                continue
            text = lines[number - 1].strip()
            found = hint_word.search(text)
            hint_text = found.group(0).lower() if found else "-"
            rows.append((kind, sources[number], hint_text, path, number, text.replace("\t", " ")))

    for old, new in touched_entries(repo, merge_base, head):
        old_wanted, new_wanted = wanted(old), wanted(new)
        if not (old_wanted or new_wanted):
            continue
        files.update(p for p, w in ((old, old_wanted), (new, new_wanted)) if w)
        if new in untracked:
            lines = read_worktree(repo, new)
            if lines is not None and new_wanted:
                emit("added", new, lines, set(range(1, len(lines) + 1)))
            continue
        added, removed = changed_lines(repo, merge_base, head, old, new)
        if new_wanted:
            lines = read_head(new)
            if lines is not None:
                every = set(range(1, len(lines) + 1))
                emit("added", new, lines, added)
                if scope == "touched":
                    emit("existing", new, lines, every - added)
        if old_wanted and removed:
            lines = read_commit(repo, merge_base, old)
            if lines is not None:
                emit("deleted", old, lines, removed)

    hinted = {}
    for kind, _, hint_text, path, _, _ in rows:
        if hint_text != "-":
            hinted[(kind, path)] = hinted.get((kind, path), 0) + 1
    rows.sort(key=lambda r: (KIND_ORDER[r[0]], -hinted.get((r[0], r[3]), 0), r[3], r[4]))
    return merge_base, sorted(files), sorted(generated), rows


def spell(path):
    """A path as the output prints it: as is, or, when it holds a control
    character, a backslash or starts with a quote, in double quotes with C
    escapes, the way Git quotes a name."""
    if not any(ord(c) < 0x20 or c in '\\\x7f' for c in path) and not path.startswith('"'):
        return path
    escapes = {"\t": "\\t", "\n": "\\n", "\r": "\\r", "\\": "\\\\", '"': '\\"'}
    body = "".join(escapes.get(c, f"\\{ord(c):03o}" if ord(c) < 0x20 or c == "\x7f" else c) for c in path)
    return f'"{body}"'


def render(rows, fmt):
    if fmt == "md":
        out = ["| kind | source | hint | location | text |", "|---|---|---|---|---|"]
        for kind, source, hint_text, path, number, text in rows:
            cell = text.replace("|", "\\|")
            out.append(f"| {kind} | {source} | {hint_text} | {spell(path)}:{number} | {cell} |")
        return "\n".join(out) + "\n"
    return "".join(
        f"{kind}\t{source}\t{hint_text}\t{spell(path)}:{number}\t{text}\n"
        for kind, source, hint_text, path, number, text in rows
    )


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--repo", default=".")
    parser.add_argument("--base", default="main")
    parser.add_argument("--head", help="a commit; the working tree when absent")
    parser.add_argument("--scope", choices=["changed", "touched"], default="touched")
    parser.add_argument("--paths", nargs="*", default=DEFAULT_PATHS)
    parser.add_argument("--exclude", nargs="*", default=DEFAULT_EXCLUDE)
    parser.add_argument("--recording", nargs="*", default=[], help="a domain recording with a name of its own")
    parser.add_argument("--hint", default=DEFAULT_HINT, help="words that order the reading")
    parser.add_argument("--format", choices=["tsv", "md"], default="tsv")
    parser.add_argument("--out")
    parser.add_argument("--files", action="store_true", help="print touched files only")
    args = parser.parse_args()

    repo = os.path.abspath(args.repo)
    for ref in (args.base, args.head):
        if ref:
            git(repo, "rev-parse", "--verify", "--quiet", ref + "^{commit}")
    merge_base, files, generated, rows = collect(
        repo, args.base, args.head, args.scope, args.paths, args.exclude,
        set(args.recording), args.hint,
    )

    # A path that is not UTF-8 is written back as the bytes it was read as.
    sys.stdout.reconfigure(errors="surrogateescape")
    sys.stderr.reconfigure(errors="backslashreplace")
    if args.files:
        sys.stdout.write("".join(f"{spell(path)}\n" for path in files))
        return

    text = render(rows, args.format)
    if args.out:
        with open(args.out, "w", encoding="utf-8", errors="surrogateescape") as handle:
            handle.write(text)
    else:
        sys.stdout.write(text)
    by_kind = {kind: sum(1 for row in rows if row[0] == kind) for kind in KIND_ORDER}
    by_source = {}
    for row in rows:
        by_source[row[1]] = by_source.get(row[1], 0) + 1
    hinted = sum(1 for row in rows if row[2] != "-")
    sources = ", ".join(f"{count} {name}" for name, count in sorted(by_source.items()))
    sys.stderr.write(
        f"inventory: merge base {merge_base[:12]}, head {args.head or 'working tree'}; "
        f"{len(files)} files touched, {len(rows)} rows "
        f"({by_kind['added']} added, {by_kind['deleted']} deleted, {by_kind['existing']} existing; "
        f"{sources or 'none'}; {hinted} hinted)\n"
    )
    if generated:
        sys.stderr.write(f"inventory: {len(generated)} generated files have no rows: {', '.join(generated)}\n")


if __name__ == "__main__":
    main()
