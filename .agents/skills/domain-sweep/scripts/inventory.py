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
in double quotes with C escapes, as Git prints it. In the text, a tab reads
as a space and any other control character as a two-digit hex
escape, backslash x then the code.
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
import copy
import fnmatch
import io
import os
import re
import subprocess
import sys
import tempfile
import tokenize
import unicodedata

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

# The whole of a comment that declares its file generated, after the Go
# convention: "Code generated <by what> DO NOT EDIT." A comment that only
# mentions the phrase declares nothing.
GENERATED = re.compile(r"Code generated .* DO NOT EDIT\.?")

# Two words in a one-line literal: a message or a sentence, not an import
# path, a key, a format verb or a struct tag.
WORDS = re.compile(r"[A-Za-z]{2,}[ \t]+[A-Za-z]{2,}")

# Go and Python test declarations, matched against a line's code alone:
# every literal on the line reads "", so a declaration written inside a
# fixture string is not one. ts_sources follows TypeScript test calls token
# by token.
# A Go declaration starts a line or follows a semicolon: func helper() {}; func TestX(.
GO_TEST = re.compile(r"(?:^|;)\s*func\s+Test\w*\s*\(")
GO_TEST_NAME = re.compile(r"^\s*Test\w*\s*\(")
GO_FUNC_END = re.compile(r"(?:^|;)\s*func\s*$")
PY_TEST_CLASS = re.compile(r"^\s*class\s+Test\w*\b")
PY_TEST_DEF = re.compile(r"^\s*(?:async\s+)?def\s+test")


def py_test_declaration(statement):
    """Whether a Python statement's code declares a test: class Test..., or
    def test_...( with any PEP 695 type parameters, def test_x[T](, before
    the parenthesis."""
    if PY_TEST_CLASS.match(statement):
        return True
    head = PY_TEST_DEF.match(statement)
    if not head:
        return False
    i = name_end(statement, head.end())
    while i < len(statement) and statement[i].isspace():
        i += 1
    if statement[i:i + 1] == "[":
        level = 0
        while i < len(statement):
            level += {"[": 1, "]": -1}.get(statement[i], 0)
            i += 1
            if level == 0:
                break
        while i < len(statement) and statement[i].isspace():
            i += 1
    return statement[i:i + 1] == "("
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


# A file the inventory reads: a regular file, executable or not. Links and
# submodules hold no prose.
REGULAR_MODES = ("100644", "100755")
NULL_OID = re.compile(r"^0+$")

# Settings that would move hunk positions off the files' own lines are
# turned off: text conversion, external diff, joined hunks, and an
# attribute that calls a text file binary.
HUNK_OPTIONS = ("-U0", "--inter-hunk-context=0", "--no-color", "--text", "--no-ext-diff", "--no-textconv")


class Entry:
    """One file the change touches, from git diff --raw: its path, blob and
    mode on each side. old is None for an added file, new for a deleted
    one; a rename carries both paths."""

    def __init__(self, old, new, old_blob=None, new_blob=None, old_mode="000000", new_mode="000000"):
        self.old, self.new = old, new
        self.old_blob, self.new_blob = old_blob, new_blob
        self.old_regular = old is not None and old_mode in REGULAR_MODES
        self.new_regular = new is not None and new_mode in REGULAR_MODES


def touched_entries(repo, base, head):
    """The files the change touches. Paths and blobs come from -z raw
    output, never from a diff header or a pathspec."""
    target = [head] if head else []
    fields = git_paths(repo, "diff", "--raw", "-z", "--no-abbrev", "--no-relative", "-M", base, *target, "--")
    entries = []
    i = 0
    while i < len(fields):
        old_mode, new_mode, old_blob, new_blob, status = fields[i].lstrip(":").split(" ")
        if status[0] in "RC":
            old, new = (fields[i + 1] if status[0] == "R" else None), fields[i + 2]
            i += 3
        else:
            path = fields[i + 1]
            old, new = (path if status[0] != "A" else None), (path if status[0] != "D" else None)
            i += 2
        if status[0] == "C":
            old_blob, old_mode = None, "000000"
        entries.append(Entry(
            old, new, None if NULL_OID.match(old_blob) else old_blob,
            None if NULL_OID.match(new_blob) else new_blob, old_mode, new_mode,
        ))
    if not head:
        # Against the working tree, a new side's hash may name content Git
        # has not stored, a staged rename edited since: read the file then.
        for entry in entries:
            if entry.new_blob is not None and not blob_stored(repo, entry.new_blob):
                entry.new_blob = None
        for path in git_paths(repo, "ls-files", "-z", "--others", "--exclude-standard"):
            entries.append(Entry(None, path, new_mode="100644"))
    return entries


def blob_stored(repo, blob):
    return subprocess.run(["git", "-C", repo, "cat-file", "-e", blob], capture_output=True).returncode == 0


def changed_lines(repo, entry, head):
    """The line numbers the change adds to the new file and removes from
    the old one, from a diff of exactly the two contents rows are read
    from: blob against blob for a commit, and against the working tree the
    old blob against the file's own bytes, staged or not; a file a sparse
    checkout leaves out is read from its index blob."""
    if entry.old_blob is None or entry.new is None:
        return None, None  # every line is added, or every line removed
    full = os.path.join(repo, entry.new)
    on_disk = head is None and os.path.isfile(full) and not os.path.islink(full)
    if not on_disk:
        out = git_text(repo, "diff", *HUNK_OPTIONS, entry.old_blob, entry.new_blob)
    else:
        # Both sides are plain copies, outside the repository and under
        # names no attribute matches, so no clean filter changes the file
        # being numbered: the hunks count the lines read_worktree reads.
        with tempfile.TemporaryDirectory() as tmp:
            old_copy, new_copy = os.path.join(tmp, "old"), os.path.join(tmp, "new")
            with open(old_copy, "wb") as handle:
                handle.write(git(repo, "cat-file", "blob", entry.old_blob))
            with open(os.path.join(repo, entry.new), "rb") as source, open(new_copy, "wb") as handle:
                handle.write(source.read())
            result = subprocess.run(
                ["git", "diff", "--no-index", *HUNK_OPTIONS, "--", "old", "new"],
                capture_output=True, cwd=tmp,
            )
            # --no-index exits 1 when the files differ.
            if result.returncode not in (0, 1):
                sys.stderr.write(result.stderr.decode("utf-8", "replace"))
                sys.exit(1)
            out = result.stdout.decode("utf-8", "replace")
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
            # A context line a configuration still adds advances both
            # sides; "\ No newline at end of file" advances neither.
            old_line += 1
            new_line += 1
    return added, removed


def read_worktree(repo, path):
    full = os.path.join(repo, path)
    if os.path.islink(full) or not os.path.isfile(full):
        return None
    # Read bytes: no newline translation, so a lone carriage return stays
    # inside its line, as Git keeps it.
    with open(full, "rb") as handle:
        return split_lines(decode_source(path, handle.read()))


def decode_source(path, data):
    """A file's text: UTF-8, except a Python file, which may declare its
    encoding in its first two lines (PEP 263)."""
    if path.endswith(".py"):
        try:
            encoding, _ = tokenize.detect_encoding(io.BytesIO(data).readline)
            return data.decode(encoding, "replace")
        except (SyntaxError, LookupError):
            pass
    return data.decode("utf-8", "replace")


def read_commit(repo, commit, path):
    """The lines of a regular file in a commit; None for a link, a
    submodule, or a path the commit does not hold."""
    entry = git(repo, "--literal-pathspecs", "ls-tree", "-z", commit, "--", path)
    if not entry.startswith((b"100644 ", b"100755 ")):
        return None
    result = subprocess.run(["git", "-C", repo, "cat-file", "blob", f"{commit}:{path}"], capture_output=True)
    if result.returncode != 0:
        return None
    return split_lines(decode_source(path, result.stdout))


def split_lines(text):
    """Split text into lines the way Git numbers them: at each newline and
    nowhere else, so a form feed or U+2028 inside a line keeps the
    numbering. A carriage return before the newline is dropped."""
    if text.startswith("\ufeff"):
        text = text[1:]  # a byte order mark is no part of the first line
    lines = text.split("\n")
    if lines and lines[-1] == "":
        lines.pop()
    return [line[:-1] if line.endswith("\r") else line for line in lines]


# The comment forms a generated-file marker may use, by file type: line
# comment openers, and block comment openers with their closers. In
# Markdown a "#" opens a heading, so only an HTML comment counts.
GENERATED_LINE = {"md": (), "py": ("#",), "yaml": ("#",), "yml": ("#",)}
GENERATED_BLOCK = {"md": (("<!--", "-->"),), "py": (), "yaml": (), "yml": ()}
C_LINE = ("//",)
C_BLOCK = (("/*", "*/"),)


def is_generated(path, lines):
    """A file is generated when a comment before its content says "Code
    generated ... DO NOT EDIT"; front matter at the top of a Markdown page
    is skipped first."""
    suffix = path.rsplit(".", 1)[-1] if "." in path else ""
    line_openers = GENERATED_LINE.get(suffix, C_LINE)
    blocks = GENERATED_BLOCK.get(suffix, C_BLOCK)
    i = 1 if lines and lines[0].startswith("#!") else 0  # a hashbang line
    if suffix in ("yaml", "yml") and lines and lines[0].strip() == "---":
        i = 1  # a YAML document start
    elif suffix == "md" and lines and lines[0].strip() in ("+++", "---"):
        fence = lines[0].strip()
        i = 1
        while i < len(lines) and lines[i].strip() != fence:
            i += 1
        i += 1
    openers = line_openers + tuple(opening for opening, _ in blocks)
    closers = tuple(end for _, end in blocks)

    def declares(stripped):
        body = stripped
        for opener in sorted(openers + ("*",), key=len, reverse=True):
            if body.startswith(opener):
                body = body[len(opener):]
                break
        for closer in closers:
            if body.endswith(closer):
                body = body[:-len(closer)]
        return bool(GENERATED.fullmatch(body.strip()))

    closing = None  # the token that ends the comment block being read
    for text in lines[i:]:
        stripped = text.strip()
        if (closing or (openers and stripped.startswith(openers))) and declares(stripped):
            return True
        rest = stripped
        if closing:
            if closing not in rest:
                continue
            rest = rest[rest.index(closing) + len(closing):].strip()
            closing = None
        # Read the rest of the line comment by comment, each one a place the
        # marker may stand: content after any of them means the file's
        # content has begun.
        while rest:
            if line_openers and rest.startswith(line_openers):
                if declares(rest):
                    return True
                break
            for opening, end in blocks:
                if rest.startswith(opening):
                    if end in rest[len(opening):]:
                        close = rest.index(end, len(opening)) + len(end)
                        if declares(rest[:close]):
                            return True
                        rest = rest[close:].strip()
                    else:
                        if declares(rest):
                            return True
                        closing, rest = end, ""
                    break
            else:
                return False
    return False


def continues_name(char, extra=""):
    """Whether char may continue an identifier: a letter, a digit, an
    underscore, a combining mark or a connector, as Python and JavaScript
    identifiers allow (Unicode XID_Continue), or one of extra."""
    return char in extra or ("a" + char).isidentifier()


def name_end(text, j, extra=""):
    """The index past the identifier characters from text[j]."""
    while j < len(text) and continues_name(text[j], extra):
        j += 1
    return j


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
    after_func = False  # the last code line ended with the keyword func
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
                code.append(" ")  # a comment separates tokens: func/* c */TestX(
                i += 2
                continue
            char = text[i]
            if char == "`":
                end = text.find("`", i + 1)
                if end < 0:
                    state = "raw"
                    kinds.add("text")
                    break
                # Go drops carriage returns from a raw string's value.
                if WORDS.search(text[i + 1:end].replace("\r", "")):
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
        line_code = "".join(code)
        # Go inserts no semicolon after func, so a declaration may put its
        # name on the next line.
        if GO_TEST.search(line_code) or (after_func and GO_TEST_NAME.match(line_code)):
            kinds.add("test")
        if line_code.strip():
            after_func = bool(GO_FUNC_END.search(line_code))
        if kinds and text.strip():
            found[number] = pick(kinds)
    return found


# A slash opens a regular expression literal where an expression may
# start: after an operator or punctuation, after one of these keywords, or
# after the parenthesis that closes a condition (if (x) /re/.test(s)).
# After an operand it divides. The lexer keeps the previous token across
# lines and comments, never the raw text before the slash.
REGEX_KEYWORDS = {
    "return", "typeof", "case", "do", "else", "in", "new", "delete", "void", "throw", "yield", "await",
    "instanceof", "default", "extends",
    # A statement ends after these, by automatic semicolon insertion when
    # nothing follows on their line, so a slash after them starts one.
    "debugger", "break", "continue",
}
CONTROL_KEYWORDS = {"if", "while", "for", "with"}
# A brace after one of these words opens an object literal, an operand
# once closed; after any other word it opens a block (else {, class A {).
OBJECT_AFTER_WORDS = {
    "return", "typeof", "yield", "await", "void", "delete", "throw", "new", "in", "of", "instanceof", "case", "default",
}
# A brace after one of these opens a block; after any other operator it
# opens an object literal. None is the start of the file.
BLOCK_AFTER = {")", ";", "{", "}", "=>", ">", None}


# Keywords after which a function or class stands in an expression; default
# (export default function f) and the statement keywords do not.
EXPRESSION_KEYWORDS = REGEX_KEYWORDS - {"default", "else", "do", "debugger", "break", "continue"}


def ends_operand(prev):
    """Whether the previous token ends an operand: a name that is not a
    keyword in its place, a number, a literal, or a closing bracket.
    Tokens are ("word", name); ("ident", name) for a keyword's spelling
    the file binds as a name (const await = 4); ("keyword", name) for a
    name in a keyword role; "operand"; and "operator"."""
    if isinstance(prev, tuple):
        return prev[0] == "ident" or (prev[0] == "word" and prev[1] not in REGEX_KEYWORDS)
    return prev == "operand"


def in_expression(prev, last_op, after_label_colon):
    """Whether a token after prev stands where an expression is expected,
    so that a function or class there is an expression."""
    if isinstance(prev, tuple):
        return prev[0] == "keyword" and prev[1] == "of" or (prev[0] == "word" and prev[1] in EXPRESSION_KEYWORDS)
    if prev == "operand" or after_label_colon:
        return False
    return last_op not in BLOCK_AFTER


TEST_CALLEES = {"it", "test", "describe"}
TS_ESCAPE = re.compile(r"\\u(?:([0-9A-Fa-f]{4})|\{([0-9A-Fa-f]+)\})")


# Unicode's Other_ID_Start and Other_ID_Continue, which ECMAScript
# identifiers include by ID_Start and ID_Continue (Python's NFKC-closed XID
# sets leave some out, such as U+309B).
OTHER_ID_START = set("\u1885\u1886\u2118\u212e\u309b\u309c")
OTHER_ID_CONTINUE = set("\u00b7\u0387\u1369\u136a\u136b\u136c\u136d\u136e\u136f\u1370\u1371\u19da\u200c\u200d\u30fb\uff65")
ID_START_CATEGORIES = {"Lu", "Ll", "Lt", "Lm", "Lo", "Nl"}
ID_CONTINUE_CATEGORIES = ID_START_CATEGORIES | {"Mn", "Mc", "Nd", "Pc"}


def starts_ts_name(char):
    """Whether char may start an ECMAScript identifier: $, _, or a Unicode
    ID_Start character."""
    return char in "_$" or char in OTHER_ID_START or unicodedata.category(char) in ID_START_CATEGORIES


def continues_ts_name(char):
    """Whether char may continue an ECMAScript identifier: $, ZWNJ, ZWJ, or
    a Unicode ID_Continue character."""
    return (char == "$" or char in OTHER_ID_START or char in OTHER_ID_CONTINUE
            or unicodedata.category(char) in ID_CONTINUE_CATEGORIES)


def ts_name(text, i):
    """Read a TypeScript identifier from text[i], its Unicode escapes
    decoded: t\\u0065st is test. Return the index past it and the name.
    The first character is always consumed."""
    word = []
    j = i
    while j < len(text):
        escape = TS_ESCAPE.match(text, j)
        if escape:
            code = int(escape.group(1) or escape.group(2), 16)
            word.append(chr(code) if code <= 0x10FFFF else "\ufffd")
            j = escape.end()
        elif j == i or continues_ts_name(text[j]):
            word.append(text[j])
            j += 1
        else:
            break
    return j, "".join(word)

QUOTED = re.compile(r"""(["'`])((?:\\.|(?!\1).)*)\1""")
SWALLOWED_TEST = re.compile(r"""(?:^|[^\w.$])(?:it|test|describe)(?:\.\w+)*\s*\(\s*["'`]""")

# How many readings of one line's undecided slashes are compared at most.
MAX_READINGS = 64
# How many lines after it a line's readings are compared over, and how
# many other readings are followed at once.
SLASH_WINDOW = 30
MAX_FOLLOWED = 64


def regex_extent(text, i):
    """Read a regular expression literal opening at text[i]. Return the
    index past it, flags included; whether it closes on the line, with a
    body; and whether / or * follows the closing slash. That closing slash
    may be a comment's start that a misread slash ran into, though
    /re/*2 is a valid product."""
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
            closes = j > i + 1
            suspicious = text[j + 1:j + 2] in ("/", "*")
            j += 1
            while j < len(text) and (text[j].isalnum() or text[j] == "_"):
                j += 1
            return j, closes, suspicious
        j += 1
    return len(text), False, False


class TypeScriptLexer:
    """Reads a TypeScript or JavaScript file line by line and says which
    lines are comments, literal text or test declarations.

    Template literals may nest code in ${ }, which may nest literals; a
    stack keeps the brace depth of each open substitution.

    A slash starts a regular expression or divides, by the token before it
    (see ends_operand). Where that rule cannot see enough, the reading
    fails safe. A regular expression that does not close on its line is
    read as a division; one whose closing slash a / or * follows is read
    as a division first, since that slash may open a comment. Such a slash,
    and one whose token cannot settle it (see token), is read both ways, each reading carried over the next
    lines: the inventory goes on from the reading that leaves the fewest of
    them inside a literal or a comment, and keeps the rows every reading
    gives, so an ambiguous slash makes it read more lines, never fewer.

    A test declaration is followed token by token: a callee (it, test or
    describe, with .only, .each and the like), any type arguments, any
    table it takes in parentheses or as a tagged template, then a call
    whose first argument is a literal: that literal is the test's name,
    and every line it spans is a test row."""

    def __init__(self):
        self.state = "code"  # code, block, template, or string (a quoted literal continued past a line)
        self.quote = None  # the quote of a continued string
        self.stack = []  # brace depth in each enclosing ${ } substitution
        self.depth = 0
        self.parens = []  # per open parenthesis: [condition, for header, header had a ;, function parameters]
        # Per open brace: [role, names]. The role is "object", "body" (of a
        # function in an expression, or an inline type after as or
        # satisfies), "type" (a type alias's literal), "return type" or
        # "block"; names are the keyword spellings bound as names inside it.
        self.braces = []
        self.prev = "operator"  # the previous token, as ends_operand reads it
        self.before = "operator"  # the token before prev
        self.last_op = None  # the text of the last operator token
        self.last_op_before = None  # last_op as it was one token earlier
        self.case_label = False  # a case or default waits for its colon
        self.after_case_colon = False  # the last token is the colon of a case or statement label
        self.jump_line = 0  # the line a break or continue was read on
        self.prev_line = 0  # the logical line the previous token was read on
        # Logical lines: Git's lines, each split again at a carriage return,
        # U+2028 or U+2029, which end a TypeScript line too.
        self.logical = 0
        self.prev_sure = True  # a slash after the previous token is surely read right (see token)
        self.bound = set()  # keyword spellings bound as names outside every brace: const await
        self.parameter_names = set()  # names a function's parameters bind, for its body
        # A function or class in an expression: False; "params", waiting for
        # its parameter list; ("paren", n), inside it, the n-th open; "body",
        # waiting for the body brace; or "return", reading a return type.
        self.expression_body = False
        self.type_alias = False  # a type alias declaration is being read
        self.type_arguments = 0  # angle brackets open after as or satisfies
        self.type_context = False  # a type follows as or satisfies: Units.Count<number>
        self.after_dot = False  # prev was a member access dot: the next word is a property
        # The test declaration being read: None; "callee"; ("open", n), just
        # after the call's parenthesis, the n-th open; ("args", n), inside a
        # call whose first argument is no literal; ("tagged", t), inside a
        # tagged template table, the t-th open template; ("generic", n),
        # inside type arguments, it.each<Row>(...), n angle brackets deep;
        # ("name", "string", n) or ("name", t, n), inside the name literal
        # of the call that opened the n-th parenthesis.
        self.test_call = None
        # Calls whose arguments a nested test call interrupted, describe(
        # suiteName, () => { test("...") }), to resume when it is read.
        self.test_outer = []
        self.callee_line = 0  # the line the test callee began on

    def drop_call(self):
        """The test call being read turned out to be none, or ended: resume
        the call it interrupted, if any."""
        if self.test_outer:
            self.test_call, self.callee_line = self.test_outer.pop()
        else:
            self.test_call = None

    def signature(self):
        """What decides how the lexer reads the lines to come: two lexers
        with the same signature read them alike."""
        return repr((
            self.state, self.quote, self.stack, self.depth, self.parens, self.braces, self.prev,
            self.prev_line == self.logical,
            self.prev_sure, self.test_call, self.test_outer, self.type_alias, self.type_arguments,
            self.type_context, self.expression_body, self.case_label, self.bound,
        ))

    def snapshot(self):
        return copy.deepcopy(self.__dict__)

    def restore(self, snapshot):
        self.__dict__.update(copy.deepcopy(snapshot))

    def literals_open(self):
        """How many literals are open: templates, and a continued string."""
        return len(self.stack) + (1 if self.state in ("template", "string") else 0)

    def sources(self, lines):
        """Map line number to source.

        One line cannot tell a misread regular expression from a division
        before a literal that runs on; the lines after it can. So every
        reading of an undecided slash is followed: the lexers that follow
        them read each line in step, each one branching again at a slash it
        cannot settle, and a line's rows are those of every reading, so a
        wrong guess reads more lines, never fewer. Readings whose states
        meet merge into one. Reversing one slash may reveal another
        undecided one, so a line's readings are explored breadth first,
        each adding one reversal of a slash it found, up to MAX_READINGS;
        and when more than MAX_FOLLOWED readings are open, those that leave
        the most of the next SLASH_WINDOW lines inside a literal or a
        comment, or damaged, are dropped."""
        found = {}
        readers = [(0, self)]  # (window cost when it branched, lexer)
        for number, text in enumerate(lines, start=1):
            kinds, marks = set(), []
            following = {}  # signature -> (cost, lexer)
            for cost, lexer in readers:
                # Only a line with a slash has a decision to reverse.
                start = lexer.snapshot() if "/" in text else None
                read = lexer.read(number, text, frozenset())
                if not read[2]:
                    kinds |= read[0]
                    marks += read[1]
                    keep(following, lexer, cost)
                    continue
                queue = [frozenset()]
                explored = {frozenset()}
                readings = 0
                while queue and readings < MAX_READINGS:
                    reversed_slashes = queue.pop(0)
                    reading = TypeScriptLexer()
                    reading.restore(start)
                    result = reading.read(number, text, reversed_slashes)
                    readings += 1
                    kinds |= result[0]
                    marks += result[1]
                    for position in result[2]:
                        grown = reversed_slashes | {position}
                        if position not in reversed_slashes and grown not in explored:
                            explored.add(grown)
                            queue.append(grown)
                    after = reading.snapshot()
                    reading_cost = reading.window_cost(lines, number, result[3])
                    reading.restore(after)
                    keep(following, reading, reading_cost)
            readers = sorted(following.values(), key=lambda pair: pair[0])[:MAX_FOLLOWED]
            for line in marks:
                found[line] = "test"
            if kinds and text.strip():
                found[number] = pick(kinds)
        return found

    def window_cost(self, lines, number, damaged):
        """Read on from the state after line number for SLASH_WINDOW lines
        and count the lines that end inside a literal or a comment, a
        damaged line counting three. The caller restores the state."""
        cost = 3 if damaged else 0
        cost += 1 if self.state in ("template", "string", "block") else 0
        for later in range(number + 1, min(len(lines), number + SLASH_WINDOW) + 1):
            _, _, _, later_damaged = self.read(later, lines[later - 1], frozenset())
            cost += 3 if later_damaged else 0
            cost += 1 if self.state in ("template", "string", "block") else 0
        return cost

    def read(self, number, text, reversed_slashes):
        """Read one line. Return its kinds, the earlier lines it marks as
        test rows, the positions of its slash decisions that could have
        gone the other way, and whether the reading shows damage: a quoted
        string left open, or a comment opened where a slash read as
        division would have been a regular expression running past it.
        Every /* and // holds a slash that ends a regular expression, so
        only a misread one, [/*], can hold a comment opener."""
        self.logical += 1
        kinds = set()
        marks = []
        slashes = []
        divisions = []  # (slash, end of the regular expression it could open)
        damaged = False
        if self.state in ("template", "string"):
            kinds.add("text")
        elif self.state == "block":
            kinds.add("comment")
        if isinstance(self.test_call, tuple) and self.test_call[0] == "name":
            kinds.add("test")
        segment = []  # the template characters read on this line

        def end_segment():
            if WORDS.search("".join(segment)):
                kinds.add("text")
            segment.clear()

        def token(kind, op=None, sure=None):
            """Record a token. sure says whether a slash after it is surely
            read right: after a literal, a number, a plain name or a closing
            parenthesis or square bracket it divides, after most operators
            it starts a regular expression; after a closing brace, >, a
            postfix operator or a name spelled like a keyword, the token's
            role decides, and the rule may guess wrong."""
            if sure is None:
                if kind == "operator":
                    sure = op not in ("}", ">")
                elif isinstance(kind, tuple):
                    sure = kind[0] == "word" and kind[1] not in ("await", "yield", "of")
                else:
                    sure = False
            self.before, self.prev, self.after_dot, self.prev_line = self.prev, kind, False, self.logical
            self.prev_sure = sure
            self.last_op_before, self.last_op = self.last_op, (op if kind == "operator" else None)
            self.after_case_colon = False

        def templates_open():
            return len(self.stack) + (1 if self.state == "template" else 0)

        def literal_starts(continues):
            """A string or template opens in code; continues names how the
            literal is followed: "string" or the template's open count."""
            if isinstance(self.test_call, tuple) and self.test_call[0] == "open":
                self.test_call = ("name", continues, self.test_call[1])
                kinds.add("test")
                if self.callee_line != number:
                    marks.append(self.callee_line)
            elif self.test_call == "callee" and continues != "string":
                self.test_call = ("tagged", continues)
            elif self.test_call == "callee":
                self.drop_call()

        def literal_ends(closed):
            """A string or template closed; closed is "string" or the open
            count it had."""
            call = self.test_call
            if isinstance(call, tuple) and call[:2] == ("name", closed):
                # The rest of the call's arguments; a chained call after it,
                # test.skipIf("win32" === os)("name", ...), may name a test too.
                self.test_call = ("args", call[2])
            elif self.test_call == ("tagged", closed):
                self.test_call = "callee"

        def other_token():
            """Any token that is not part of a test declaration's shape."""
            if self.test_call == "callee":
                self.drop_call()
            elif isinstance(self.test_call, tuple) and self.test_call[0] == "open":
                self.test_call = ("args", self.test_call[1])

        i = 0
        while i < len(text):
            char = text[i]
            if self.state == "block":
                end = text.find("*/", i)
                if end < 0:
                    break
                self.state, i = "code", end + 2
                continue
            if self.state == "string":
                i, _, how = scan_quoted(text, i, self.quote)
                if how != "closed":
                    if how == "open":
                        self.state = "code"
                        literal_ends("string")
                    break
                self.state = "code"
                literal_ends("string")
                token("operand", sure=True)
                continue
            if self.state == "template":
                if char == "\\":
                    segment.append(text[i:i + 2])
                    i += 2
                    continue
                if char == "`":
                    end_segment()
                    closed = templates_open()
                    self.state = "code"
                    literal_ends(closed)
                    token("operand", sure=True)
                    i += 1
                    continue
                if text[i:i + 2] == "${":
                    end_segment()
                    self.stack.append(self.depth)
                    self.depth, self.state = 0, "code"
                    token("operator")
                    i += 2
                    continue
                segment.append(char)
                i += 1
                continue
            pair = text[i:i + 2]
            if pair in ("//", "/*") and any(slash < i < end for slash, end in divisions):
                damaged = True
            if pair == "//":
                kinds.add("comment")
                # A carriage return, U+2028 or U+2029 ends the comment too;
                # Git's line goes on after it.
                ends = [k for k in (text.find(c, i + 2) for c in "\r\u2028\u2029") if k >= 0]
                if not ends:
                    break
                self.logical += 1
                i = min(ends) + 1
                continue
            if pair == "/*":
                self.state = "block"
                kinds.add("comment")
                i += 2
                continue
            if char.isspace():
                if char in "\r\u2028\u2029":
                    self.logical += 1
                i += 1
                continue
            if number == 1 and i == 0 and pair == "#!":
                break  # a hashbang line
            if char == "#" and i + 1 < len(text) and (starts_ts_name(text[i + 1]) or text.startswith("\\u", i + 1)):
                # A private name, #count or #\\u0069n, is a member: an operand.
                j, _ = ts_name(text, i + 1)
                other_token()
                token("operand", sure=True)
                i = j
                continue
            if starts_ts_name(char) or text.startswith("\\u", i):
                j, word = ts_name(text, i)
                self.word(word, number, token, other_token)
                i = j
                continue
            if char.isdigit():
                j = i
                while j < len(text) and (text[j].isalnum() or text[j] in "_."):
                    j += 1
                other_token()
                token("operand", sure=True)
                i = j
                continue
            if char == "`":
                self.state = "template"
                literal_starts(templates_open())
                i += 1
                continue
            if char in "\"'":
                literal_starts("string")
                i, content, how = scan_quoted(text, i + 1, char)
                if WORDS.search(content):
                    kinds.add("text")
                if how == "continued":
                    self.state, self.quote = "string", char
                    kinds.add("text")
                    break
                if how == "open":
                    damaged = True
                literal_ends("string")
                token("operand", sure=True)
                continue
            if char == "/":
                end, closes, suspicious = regex_extent(text, i)
                wanted = not ends_operand(self.prev)
                plausible = closes and not suspicious
                regex = wanted and plausible
                if closes and ((wanted and suspicious) or not (self.prev_sure and self.prev_line == self.logical)):
                    # Either reading could be meant, and neither the token
                    # before nor the literal's end settles it: note the
                    # decision. A regular expression that a comment opener
                    # would end reads as a division first.
                    slashes.append(i)
                    if i in reversed_slashes:
                        regex = not regex
                other_token()
                if regex:
                    body = text[i + 1:end]
                    if SWALLOWED_TEST.search(body):
                        # A test call inside a regular expression is one the
                        # slash rule misread: keep its line as a test.
                        kinds.add("test")
                    elif any(WORDS.search(m.group(2)) for m in QUOTED.finditer(body)):
                        # So is a quoted phrase, likely a string: read the line.
                        kinds.add("text")
                    token("operand", sure=True)
                    i = end
                else:
                    if plausible:
                        divisions.append((i, end))
                    token("operator", "/")
                    i += 1
                continue
            if pair == "=>":
                other_token()
                token("operator", "=>")
                i += 2
                continue
            if text[i:i + 3] == "...":
                other_token()
                token("operator")
                i += 3
                continue
            if char == "." or pair == "?.":
                if self.test_call != "callee":
                    other_token()
                i += 1 if char == "." else 2
                self.before, self.prev, self.after_dot = self.prev, "operator", True
                continue
            if pair in ("++", "--"):
                # After an operand on the same line, ++ and -- are postfix
                # and end it; before one, or after a line break, prefix.
                other_token()
                token("operand" if ends_operand(self.prev) and self.prev_line == self.logical else "operator")
                i += 2
                continue
            if char == "!" and ends_operand(self.prev) and self.prev_line == self.logical and text[i + 1:i + 2] != "=":
                # A non-null assertion, count!, ends an operand; it takes no
                # line break before it.
                other_token()
                token("operand")
                i += 1
                continue
            self.punctuation(char, text, i, token, other_token, templates_open)
            if self.state == "template":
                i += 1
                continue
            i += 1
        if self.state == "template":
            # The literal goes on past this line: every line it spans is text.
            kinds.add("text")
        end_segment()
        return kinds, marks, slashes, damaged

    def word(self, word, number, token, other_token):
        """Read a name, a keyword or a property."""
        prev = self.prev
        if word in ("as", "satisfies") and not self.after_dot:
            self.type_context = True
        if self.after_dot:
            # A property: counts.in is an operand, and test.each stays a
            # test callee.
            if self.test_call != "callee":
                other_token()
            token("operand")
            return
        call = self.test_call
        if word in TEST_CALLEES and (
            call in (None, "callee") or (isinstance(call, tuple) and call[0] in ("open", "args", "tagged"))
        ):
            if isinstance(call, tuple):
                # A test inside another call's arguments, its first one
                # included, describe(it("...").title, ...), or inside a
                # tagged table's substitution.
                outer = ("args", call[1]) if call[0] == "open" else call
                self.test_outer.append((outer, self.callee_line))
            self.test_call, self.callee_line = "callee", number
        else:
            other_token()
        if prev in (("word", "break"), ("word", "continue")) and self.jump_line == self.logical:
            # A label: the statement ends after it, as after the break or
            # continue itself.
            token(("keyword", prev[1]))
        elif word in ("await", "yield") and (
            word in self.bound or any(word in names for _, names in self.braces)
            or prev in (("word", "const"), ("word", "let"), ("word", "var"))
            or (self.parens and self.parens[-1][3])
        ):
            # Outside an async function or a generator, a script may name a
            # variable or parameter so; the name lives in its own scope.
            if self.parens and self.parens[-1][3]:
                self.parens[-1][4].add(word)
            elif prev in (("word", "const"), ("word", "let"), ("word", "var")):
                (self.braces[-1][1] if self.braces else self.bound).add(word)
            token(("ident", word))
        elif word == "of":
            # of is a keyword only where a for header's binding ends:
            # for (const x of xs), never for (i = of / 2;).
            binding = (isinstance(prev, tuple) and prev[0] in ("word", "ident")
                       and prev[1] not in ("let", "const", "var")) or prev == "operand"
            header = self.parens and self.parens[-1][1] and not self.parens[-1][2]
            token(("keyword" if header and binding else "word", word))
        else:
            if word in ("break", "continue"):
                self.jump_line = self.logical
            elif word in ("case", "default"):
                self.case_label = True
            elif word in ("function", "class") and (
                in_expression(prev, self.last_op, self.after_case_colon) if prev != ("word", "async")
                else in_expression(self.before, self.last_op_before, False)
            ):
                # In an expression its body, once closed, is an operand; a
                # function's body follows its parameter list.
                self.expression_body = "params" if word == "function" else "body"
            elif word == "type" and not ends_operand(prev) and not in_expression(
                prev, self.last_op, self.after_case_colon
            ):
                self.type_alias = True
            token(("word", word))

    def punctuation(self, char, text, i, token, other_token, templates_open):
        """Read one bracket or operator character."""
        prev = self.prev
        in_type = self.type_context
        if char not in "<|&[]" and not self.type_arguments:
            # A type after as or satisfies runs through names, dots, unions,
            # intersections and array brackets: number | Box<number>[].
            self.type_context = False
        if char == "(":
            header = isinstance(prev, tuple) and prev[1] == "for" or (
                prev == ("word", "await") and self.before == ("word", "for")
            )
            condition = header or (isinstance(prev, tuple) and prev[1] in CONTROL_KEYWORDS)
            # The parameters of a function declaration or expression.
            params = prev == ("word", "function") or (
                isinstance(prev, tuple) and prev[0] == "word" and self.before == ("word", "function")
            )
            self.parens.append([condition, header, False, params, set()])
            if self.expression_body == "params":
                self.expression_body = ("paren", len(self.parens))
            if self.test_call == "callee":
                self.test_call = ("open", len(self.parens))
            else:
                other_token()
            token("operator", "(")
        elif char == ")":
            opened = len(self.parens)
            closed = self.parens.pop() if self.parens else [False, False, False, False, set()]
            closes_condition = closed[0]
            if closed[3]:
                self.parameter_names = closed[4]
            if self.expression_body == ("paren", opened):
                self.expression_body = "body"
            call = self.test_call
            if isinstance(call, tuple) and call[0] in ("open", "args") and call[1] == opened:
                self.test_call = "callee"
            elif any(outer == ("args", opened) for outer, _ in self.test_outer):
                # The interrupted call closes around an unfinished one.
                while self.test_outer and self.test_outer[-1][0] != ("args", opened):
                    self.test_outer.pop()
                _, self.callee_line = self.test_outer.pop()
                self.test_call = "callee"
            else:
                other_token()
            if closes_condition:
                token("operator", ")")
            else:
                token("operand", sure=True)
        elif char == "]":
            other_token()
            token("operand", sure=True)
        elif char == "{":
            if self.expression_body == "return" and self.last_op == ":":
                opens = "return type"  # function(): { value: number } {
                self.braces.append([opens, set()])
                other_token()
                self.depth += 1
                token("operator", "{")
                return
            if self.expression_body in ("body", "return"):
                opens, self.expression_body = "body", False
            elif in_type:
                opens = "body"  # an inline type, 4 satisfies { ... }: an operand once closed
            elif self.type_alias:
                opens = "type"
            elif self.parens and self.parens[-1][1]:
                opens = "object"  # a destructuring pattern in a for header
            elif isinstance(prev, tuple):
                opens = "object" if prev[0] != "ident" and prev[1] in OBJECT_AFTER_WORDS else "block"
            elif prev == "operand" or self.after_case_colon:
                opens = "block"
            else:
                opens = "block" if self.last_op in BLOCK_AFTER else "object"
            self.braces.append([opens, self.parameter_names])
            self.parameter_names = set()
            other_token()
            self.depth += 1
            token("operator", "{")
        elif char == "}":
            if self.depth == 0 and self.stack:
                # A substitution ends: a test call read inside it is over,
                # and a tagged table it interrupted resumes.
                if self.test_call == "callee":
                    self.drop_call()
                self.depth = self.stack.pop()
                self.state = "template"
                return
            other_token()
            self.depth -= 1
            closes = self.braces.pop()[0] if self.braces else "block"
            if closes == "type" and not any(role == "type" for role, _ in self.braces):
                # The type alias ends with its literal, as a statement.
                self.type_alias = False
                token("operator", "}")
            elif closes in ("object", "body", "type", "return type"):
                token("operand")
            else:
                token("operator", "}")
        elif char == "<" and self.test_call == "callee":
            self.test_call = ("generic", 1)
            token("operator")
        elif char in "<>" and isinstance(self.test_call, tuple) and self.test_call[0] == "generic":
            level = self.test_call[1] + (1 if char == "<" else -1)
            self.test_call = ("generic", level) if level else "callee"
            token("operator")
        elif char == ":" and (self.case_label or (
            isinstance(prev, tuple) and prev[0] in ("word", "ident")
            and (not self.braces or self.braces[-1][0] != "object")
            and self.before == "operator" and self.last_op_before in BLOCK_AFTER
        )):
            # The colon of a case label or of a statement label: a brace
            # after it opens a block.
            other_token()
            token("operator", ":")
            self.case_label, self.after_case_colon = False, True
        elif char == ":" and self.expression_body == "body":
            self.expression_body = "return"  # function(): T { ... }
            other_token()
            token("operator", ":")
        elif char == "<" and (isinstance(prev, tuple) or prev == "operand") and (
            self.type_arguments or self.type_context
        ):
            # Type arguments after as or satisfies: Box<number>.
            self.type_arguments += 1
            other_token()
            token("operator", "<")
        elif char == ">" and self.type_arguments:
            self.type_arguments -= 1
            if not self.type_arguments:
                self.type_context = False
            other_token()
            token("operand" if not self.type_arguments else "operator", ">")
        elif char == ";" and self.type_alias and not any(role == "type" for role, _ in self.braces):
            self.type_alias = False
            other_token()
            token("operator", ";")
        elif char == ";" and self.parens and self.parens[-1][1]:
            self.parens[-1][2] = True  # for (init; test; update): no for-of
            other_token()
            token("operator", ";")
        else:
            other_token()
            token("operator", char)


def keep(following, lexer, cost):
    """Follow lexer on, unless one in the same state already is; of two,
    the one whose reading cost less."""
    signature = lexer.signature()
    if signature not in following or cost < following[signature][0]:
        following[signature] = (cost, lexer)


def _literals_open(snapshot):
    return len(snapshot["stack"]) + (1 if snapshot["state"] in ("template", "string") else 0)


def ts_sources(lines):
    """Map line number to source for a TypeScript or JavaScript file."""
    return TypeScriptLexer().sources(lines)


PY_STRING = re.compile(r"([rRbBuUfF]{0,2})('\'\'|\"\"\"|'|\")")
PY_OPEN = "([{"
PY_CLOSE = ")]}"


def python_sources(lines):
    """Map line number to source for a Python file. F-strings are read as
    Python 3.12 reads them: the literal parts are text, and the code in a
    { } replacement field is code, comments and nested strings included,
    up to a format spec after a top-level colon, which is text again. A
    stack holds what is open: ["string", quote, raw], ["fstring", quote,
    raw] (an f-string's literal part), ["field", depth] (a replacement
    field, with its bracket depth) and ["spec"] (a format spec)."""
    found = {}
    stack = []
    joined = []  # code of the lines of a statement that goes on
    joined_lines = []  # the lines that statement spans so far
    joined_lines_of_pieces = []  # the line of each piece in joined
    brackets = 0  # brackets the statement's code leaves open
    for number, text in enumerate(lines, start=1):
        kinds = set()
        code = []  # the line's code outside every literal, for declarations
        segment = []  # the literal characters read on this line

        def end_segment():
            if WORDS.search("".join(segment)):
                kinds.add("text")
            segment.clear()

        def in_literal():
            return bool(stack) and stack[-1][0] in ("string", "fstring", "spec")

        if in_literal():
            kinds.add("text")
        continued = False
        i = 0
        while i < len(text):
            char = text[i]
            top = stack[-1] if stack else None
            if top is not None and top[0] in ("string", "fstring"):
                quote, raw = top[1], top[2]
                if char == "\\":
                    if i + 1 == len(text):
                        continued = True
                        i += 1
                        continue
                    if top[0] == "fstring" and text[i + 1] in "{}":
                        # A backslash escapes no brace: the brace still
                        # opens or closes a replacement field.
                        segment.append(char)
                        i += 1
                        continue
                    if top[0] == "fstring" and not raw and text[i + 1:i + 3] == "N{":
                        # A named escape, \N{EM DASH}, is literal text.
                        end = text.find("}", i + 3)
                        end = len(text) if end < 0 else end + 1
                        segment.append(text[i:end])
                        i = end
                        continue
                    segment.append(text[i:i + 2])
                    i += 2
                    continue
                if text.startswith(quote, i):
                    end_segment()
                    stack.pop()
                    i += len(quote)
                    continue
                if top[0] == "fstring" and char in "{}":
                    if text[i:i + 2] in ("{{", "}}"):
                        segment.append(char)
                        i += 2
                        continue
                    if char == "{":
                        end_segment()
                        stack.append(["field", 0])
                        i += 1
                        continue
                segment.append(char)
                i += 1
                continue
            if top is not None and top[0] == "spec":
                raw = next(frame[2] for frame in reversed(stack) if frame[0] == "fstring")
                if char == "\\" and not raw and text[i + 1:i + 3] == "N{":
                    # A named escape, \N{EM DASH}, is literal text here too.
                    end = text.find("}", i + 3)
                    end = len(text) if end < 0 else end + 1
                    segment.append(text[i:end])
                    i = end
                    continue
                if char == "{":
                    end_segment()
                    stack.append(["field", 0])
                elif char == "}":
                    end_segment()
                    stack.pop()  # the spec
                    stack.pop()  # its field
                else:
                    segment.append(char)
                i += 1
                continue
            # Code: at the top level, or inside a replacement field.
            if char == "#":
                kinds.add("comment")
                # A lone carriage return ends a Python line, and the comment
                # with it; Git's line goes on after it.
                end = text.find("\r", i + 1)
                if end < 0:
                    break
                code.append("\r")  # the statement ends there too
                i = end + 1
                continue
            match = PY_STRING.match(text, i) if char in "'\"rRbBuUfF" else None
            if match and (char in "'\"" or i == 0 or not (text[i - 1].isalnum() or text[i - 1] == "_")):
                prefix, quote = match.group(1).lower(), match.group(2)
                stack.append(["fstring" if "f" in prefix else "string", quote, "r" in prefix])
                if not stack[:-1]:
                    code.append(LITERAL)
                i = match.end()
                continue
            if top is not None and top[0] == "field":
                if char in PY_OPEN:
                    top[1] += 1
                elif char in PY_CLOSE:
                    if top[1] == 0 and char == "}":
                        stack.pop()
                        i += 1
                        continue
                    top[1] -= 1
                elif char == ":" and top[1] == 0:
                    stack.append(["spec"])
                i += 1
                continue
            if char.isalnum() or char == "_":
                j = name_end(text, i + 1)
                code.append(text[i:j])
                i = j
                continue
            code.append(char)
            i += 1
        if in_literal():
            # The literal goes on past this line: every line it spans is text.
            kinds.add("text")
        end_segment()
        # A one-quote literal ends with its line unless a backslash carries
        # it on; an unterminated one is closed here, as Python refuses it.
        while stack and stack[-1][0] in ("string", "fstring") and len(stack[-1][1]) == 1 and not continued:
            stack.pop()
        line_code = "".join(code)
        # A lone carriage return in code ends a Python line too, so one Git
        # line may hold several; only the last can run on into a literal.
        pieces = line_code.split("\r")
        for index, piece in enumerate(pieces):
            last = index == len(pieces) - 1
            brackets = max(0, brackets + sum(piece.count(c) for c in PY_OPEN) - sum(piece.count(c) for c in PY_CLOSE))
            explicit = not (last and stack) and piece.rstrip().endswith("\\")
            joined.append(piece.rstrip()[:-1] if explicit else piece)
            joined_lines_of_pieces.append(number)
            if not joined_lines or joined_lines[-1] != number:
                joined_lines.append(number)
            if explicit or (last and stack) or brackets:
                continue  # a backslash, an open literal or an open bracket carries the statement on
            statement = " ".join(joined)
            if py_test_declaration(statement):
                # The lines from def through the test's name.
                name_at = PY_TEST_DEF.match(statement) or PY_TEST_CLASS.match(statement)
                offset, through = 0, joined_lines[-1]
                for piece_text, line in zip(joined, joined_lines_of_pieces):
                    offset += len(piece_text) + 1
                    if offset > name_at.end():
                        through = line
                        break
                for line in joined_lines:
                    if line < through or (line == through and line != number):
                        found[line] = "test"
                if through == number:
                    kinds.add("test")
            joined, joined_lines, joined_lines_of_pieces = [], [], []
        if kinds and text.strip():
            found[number] = pick(kinds)
    return found


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


def resolve(repo, ref):
    """The commit a ref names, as an object id."""
    result = subprocess.run(
        ["git", "-C", repo, "rev-parse", "--verify", "--quiet", "--end-of-options", ref + "^{commit}"],
        capture_output=True, text=True, errors="replace",
    )
    if result.returncode != 0:
        sys.stderr.write(f"inventory: {ref} names no commit\n")
        sys.exit(1)
    return result.stdout.strip()


def find_merge_base(repo, base, head):
    result = subprocess.run(
        ["git", "-C", repo, "merge-base", "--end-of-options", base, head], capture_output=True, text=True, errors="replace",
    )
    if result.returncode != 0:
        sys.stderr.write(f"inventory: {base} and {head} have no merge base\n{result.stderr}")
        sys.exit(1)
    return result.stdout.strip()


def collect(repo, base, head, scope, globs, exclude, recordings, hint):
    hint_word = re.compile(hint, re.IGNORECASE)
    merge_base = find_merge_base(repo, base, head or "HEAD")

    def wanted(path):
        return path is not None and (matches(path, globs) or is_recording(path, recordings)) and not matches(path, exclude)

    def read_head(entry):
        if head:
            return read_commit(repo, head, entry.new)
        lines = read_worktree(repo, entry.new)
        if lines is None and entry.new_blob is not None:
            # A file a sparse checkout leaves out of the working tree: its
            # content is the blob the index holds.
            lines = split_lines(decode_source(entry.new, git(repo, "cat-file", "blob", entry.new_blob)))
        return lines

    files = set()
    generated = []
    rows = []

    def emit(kind, path, lines, chosen):
        if is_generated(path, lines):
            if path not in generated:
                generated.append(path)
            return
        sources = line_sources(path, lines, recordings)
        for number in sorted(sources):
            if number not in chosen:
                continue
            text = lines[number - 1].strip()
            found = hint_word.search(text)
            hint_text = printable(found.group(0).lower()) if found else "-"
            rows.append((kind, sources[number], hint_text, path, number, printable(text)))

    for entry in touched_entries(repo, merge_base, head):
        old, new = entry.old, entry.new
        if old is not None and new is not None and old != new and is_recording(new, recordings):
            recordings = recordings | {old}  # a recording renamed keeps its kind on the old side
        old_wanted, new_wanted = wanted(old), wanted(new)
        if not (old_wanted or new_wanted):
            continue
        files.update(p for p, w in ((old, old_wanted), (new, new_wanted)) if w)
        added, removed = changed_lines(repo, entry, head) if entry.old_regular and entry.new_regular else (None, None)
        if new_wanted and entry.new_regular:
            lines = read_head(entry)
            if lines is not None:
                every = set(range(1, len(lines) + 1))
                chosen = every if added is None else added
                emit("added", new, lines, chosen)
                if scope == "touched":
                    emit("existing", new, lines, every - chosen)
        if old_wanted and entry.old_regular and (removed is None or removed):
            lines = read_commit(repo, merge_base, old)
            if lines is not None:
                emit("deleted", old, lines, set(range(1, len(lines) + 1)) if removed is None else removed)

    hinted = {}
    for kind, _, hint_text, path, _, _ in rows:
        if hint_text != "-":
            hinted[(kind, path)] = hinted.get((kind, path), 0) + 1
    rows.sort(key=lambda r: (KIND_ORDER[r[0]], -hinted.get((r[0], r[3]), 0), r[3], r[4]))
    return merge_base, sorted(files), sorted(generated), rows


def printable(text):
    """A line's text for one output cell: a tab reads as a space, and any
    other control character, a lone carriage return among them, as a C
    escape, so a row stays one line of one cell."""
    text = text.replace("\t", " ")
    return "".join(f"\\x{ord(c):02x}" if ord(c) < 0x20 or c == "\x7f" else c for c in text)


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
            location = f"{spell(path)}:{number}".replace("|", "\\|")
            hint_cell = hint_text.replace("|", "\\|")
            out.append(f"| {kind} | {source} | {hint_cell} | {location} | {cell} |")
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

    # Paths git prints are relative to the top of the work tree, wherever
    # --repo points inside it; refs become object ids, so a ref that is
    # also a file name stays a ref.
    top = git(os.path.abspath(args.repo), "rev-parse", "--show-toplevel")
    repo = os.fsdecode(top[:-1] if top.endswith(b"\n") else top)  # the path may end in a newline of its own
    base, head = (resolve(repo, ref) if ref else None for ref in (args.base, args.head))
    merge_base, files, generated, rows = collect(
        repo, base, head, args.scope, args.paths, args.exclude, set(args.recording), args.hint,
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
