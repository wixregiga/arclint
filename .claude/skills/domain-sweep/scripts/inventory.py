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
import copy
import fnmatch
import os
import re
import subprocess
import sys
import tempfile

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
PY_TEST_DEF = re.compile(r"^\s*(?:async\s+)?def\s+test\w*\s*")


def py_test_declaration(statement):
    """Whether a Python statement's code declares a test: class Test..., or
    def test_...( with any PEP 695 type parameters, def test_x[T](, before
    the parenthesis."""
    if PY_TEST_CLASS.match(statement):
        return True
    head = PY_TEST_DEF.match(statement)
    if not head:
        return False
    i = head.end()
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
        for path in git_paths(repo, "ls-files", "-z", "--others", "--exclude-standard"):
            entries.append(Entry(None, path, new_mode="100644"))
    return entries


def changed_lines(repo, entry):
    """The line numbers the change adds to the new file and removes from
    the old one, from a diff of exactly these two files: blob against blob,
    or the old blob against the file in the working tree."""
    if entry.old_blob is None or entry.new is None:
        return None, None  # every line is added, or every line removed
    if entry.new_blob is not None:
        out = git_text(repo, "diff", *HUNK_OPTIONS, entry.old_blob, entry.new_blob)
    else:
        with tempfile.TemporaryDirectory() as tmp:
            old_copy = os.path.join(tmp, "old")
            with open(old_copy, "wb") as handle:
                handle.write(git(repo, "cat-file", "blob", entry.old_blob))
            result = subprocess.run(
                ["git", "-C", repo, "diff", "--no-index", *HUNK_OPTIONS, "--", old_copy,
                 os.path.join(repo, entry.new)],
                capture_output=True,
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
    # newline="" keeps a lone carriage return inside its line, as Git does.
    with open(full, "r", encoding="utf-8", errors="replace", newline="") as handle:
        return split_lines(handle.read())


def read_commit(repo, commit, path):
    """The lines of a regular file in a commit; None for a link, a
    submodule, or a path the commit does not hold."""
    entry = git(repo, "--literal-pathspecs", "ls-tree", "-z", commit, "--", path)
    if not entry.startswith((b"100644 ", b"100755 ")):
        return None
    result = subprocess.run(["git", "-C", repo, "cat-file", "blob", f"{commit}:{path}"], capture_output=True)
    if result.returncode != 0:
        return None
    return split_lines(result.stdout.decode("utf-8", "replace"))


def split_lines(text):
    """Split text into lines the way Git numbers them: at each newline and
    nowhere else, so a form feed or U+2028 inside a line keeps the
    numbering. A carriage return before the newline is dropped."""
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
    i = 0
    if lines and lines[0].strip() in ("+++", "---"):
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
        if closing:
            if closing in stripped:
                closing = None
            continue
        if not stripped or (line_openers and stripped.startswith(line_openers)):
            continue
        for opening, end in blocks:
            if stripped.startswith(opening):
                if end not in stripped[len(opening):]:
                    closing = end
                break
        else:
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
    "instanceof", "default",
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

QUOTED = re.compile(r"""(["'])(.*?)\1""")

# A line whose reading opened a literal it never closed is read again with
# one slash decision reversed, at most this many, from the last.
MAX_SLASH_RETRIES = 16


def regex_extent(text, i):
    """Read a regular expression literal opening at text[i]. Return the
    index past it, flags included, and whether it closes on the line with a
    closing slash that no / or * follows; a slash that does not is no
    regular expression's end but a comment's start."""
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
            plausible = j > i + 1 and text[j + 1:j + 2] not in ("/", "*")
            j += 1
            while j < len(text) and (text[j].isalnum() or text[j] == "_"):
                j += 1
            return j, plausible
        j += 1
    return len(text), False


class TypeScriptLexer:
    """Reads a TypeScript or JavaScript file line by line and says which
    lines are comments, literal text or test declarations.

    Template literals may nest code in ${ }, which may nest literals; a
    stack keeps the brace depth of each open substitution.

    A slash starts a regular expression or divides, by the token before it
    (see ends_operand). Where that rule cannot see enough, the reading
    fails safe: a regular expression that cannot end where a regular
    expression may is read as a division, and a line whose reading opens a
    string or template it never closes is read again with one slash
    decision reversed, keeping the first reading that leaves no literal
    open.

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
        # function in an expression), "type" (a type alias's literal) or
        # "block"; names are the keyword spellings bound as names inside it.
        self.braces = []
        self.prev = "operator"  # the previous token, as ends_operand reads it
        self.before = "operator"  # the token before prev
        self.last_op = None  # the text of the last operator token
        self.last_op_before = None  # last_op as it was one token earlier
        self.case_label = False  # a case or default waits for its colon
        self.after_case_colon = False  # the last token is the colon of a case or statement label
        self.jump_line = 0  # the line a break or continue was read on
        self.prev_line = 0  # the line the previous token was read on
        self.bound = set()  # keyword spellings bound as names outside every brace: const await
        self.parameter_names = set()  # names a function's parameters bind, for its body
        # A function or class in an expression: False; "params", waiting for
        # its parameter list; ("paren", n), inside it, the n-th open; or
        # "body", waiting for the body brace.
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
        # ("name", "string") or ("name", t), inside the name literal.
        self.test_call = None
        self.callee_line = 0  # the line the test callee began on

    def snapshot(self):
        return copy.deepcopy(self.__dict__)

    def restore(self, snapshot):
        self.__dict__.update(copy.deepcopy(snapshot))

    def literals_open(self):
        """How many literals are open: templates, and a continued string."""
        return len(self.stack) + (1 if self.state in ("template", "string") else 0)

    def sources(self, lines):
        """Map line number to source."""
        found = {}
        for number, text in enumerate(lines, start=1):
            # Only a line with a slash has a decision to reverse.
            start = self.snapshot() if "/" in text else None
            kinds, marks, slashes, unclosed = self.read(number, text, frozenset())
            if slashes and (unclosed or self.literals_open() > _literals_open(start)):
                for position in reversed(slashes[-MAX_SLASH_RETRIES:]):
                    self.restore(start)
                    retry = self.read(number, text, frozenset([position]))
                    if not retry[3] and self.literals_open() <= _literals_open(start):
                        kinds, marks = retry[0], retry[1]
                        break
                else:
                    self.restore(start)
                    kinds, marks, _, _ = self.read(number, text, frozenset())
            for line in marks:
                found[line] = "test"
            if kinds and text.strip():
                found[number] = pick(kinds)
        return found

    def read(self, number, text, reversed_slashes):
        """Read one line. Return its kinds, the earlier lines it marks as
        test rows, the positions of its slash decisions that could have
        gone the other way, and whether a quoted string was left open."""
        kinds = set()
        marks = []
        slashes = []
        unclosed = False
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

        def token(kind, op=None):
            self.before, self.prev, self.after_dot, self.prev_line = self.prev, kind, False, number
            self.last_op_before, self.last_op = self.last_op, (op if kind == "operator" else None)
            self.after_case_colon = False

        def templates_open():
            return len(self.stack) + (1 if self.state == "template" else 0)

        def literal_starts(continues):
            """A string or template opens in code; continues names how the
            literal is followed: "string" or the template's open count."""
            if isinstance(self.test_call, tuple) and self.test_call[0] == "open":
                self.test_call = ("name", continues)
                kinds.add("test")
                if self.callee_line != number:
                    marks.append(self.callee_line)
            elif self.test_call == "callee" and continues != "string":
                self.test_call = ("tagged", continues)
            elif self.test_call == "callee":
                self.test_call = None

        def literal_ends(closed):
            """A string or template closed; closed is "string" or the open
            count it had."""
            if self.test_call == ("name", closed):
                self.test_call = None
            elif self.test_call == ("tagged", closed):
                self.test_call = "callee"

        def other_token():
            """Any token that is not part of a test declaration's shape."""
            if self.test_call == "callee":
                self.test_call = None
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
                token("operand")
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
                    token("operand")
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
            if pair == "//":
                kinds.add("comment")
                break
            if pair == "/*":
                self.state = "block"
                kinds.add("comment")
                i += 2
                continue
            if char.isspace():
                i += 1
                continue
            if number == 1 and i == 0 and pair == "#!":
                break  # a hashbang line
            if char == "#" and i + 1 < len(text) and (text[i + 1].isalpha() or text[i + 1] in "_$"):
                # A private name, #count, is a member: an operand.
                j = i + 1
                while j < len(text) and (text[j].isalnum() or text[j] in "_$"):
                    j += 1
                other_token()
                token("operand")
                i = j
                continue
            if char.isalpha() or char in "_$":
                j = i
                while j < len(text) and (text[j].isalnum() or text[j] in "_$"):
                    j += 1
                self.word(text[i:j], number, token, other_token)
                i = j
                continue
            if char.isdigit():
                j = i
                while j < len(text) and (text[j].isalnum() or text[j] in "_."):
                    j += 1
                other_token()
                token("operand")
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
                    unclosed = True
                literal_ends("string")
                token("operand")
                continue
            if char == "/":
                end, plausible = regex_extent(text, i)
                regex = not ends_operand(self.prev) and plausible
                if plausible:
                    # Either reading could be meant: note the decision.
                    slashes.append(i)
                    if i in reversed_slashes:
                        regex = not regex
                other_token()
                if regex:
                    if any(WORDS.search(m.group(2)) for m in QUOTED.finditer(text[i + 1:end])):
                        # A quoted phrase inside a regular expression may be
                        # a string the slash rule misread: read the line.
                        kinds.add("text")
                    token("operand")
                    i = end
                else:
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
                token("operand" if ends_operand(self.prev) and self.prev_line == number else "operator")
                i += 2
                continue
            if char == "!" and ends_operand(self.prev) and self.prev_line == number and text[i + 1:i + 2] != "=":
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
        return kinds, marks, slashes, unclosed

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
        if word in TEST_CALLEES and self.test_call in (None, "callee"):
            self.test_call, self.callee_line = "callee", number
        else:
            other_token()
        if prev in (("word", "break"), ("word", "continue")) and self.jump_line == number:
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
                self.jump_line = number
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
        if char != "<" and not self.type_arguments:
            # A type after as or satisfies is names and dots up to <.
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
            else:
                other_token()
            if closes_condition:
                token("operator", ")")
            else:
                token("operand")
        elif char == "]":
            other_token()
            token("operand")
        elif char == "{":
            if self.expression_body == "body":
                opens, self.expression_body = "body", False
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
            elif closes in ("object", "body", "type"):
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
    joined_start = 0  # the first of those lines
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
                break
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
                j = i
                while j < len(text) and (text[j].isalnum() or text[j] == "_"):
                    j += 1
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
        brackets = max(0, brackets + sum(line_code.count(c) for c in PY_OPEN) - sum(line_code.count(c) for c in PY_CLOSE))
        explicit = not stack and line_code.rstrip().endswith("\\")
        joined.append(line_code.rstrip()[:-1] if explicit else line_code)
        joined_start = joined_start or number
        if not (explicit or stack or brackets):
            # The statement ends with this line: a backslash, an open
            # literal or an open bracket would carry it on.
            if py_test_declaration(" ".join(joined)):
                if joined_start != number:
                    found[joined_start] = "test"
                else:
                    kinds.add("test")
            joined, joined_start = [], 0
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

    def read_head(path):
        return read_commit(repo, head, path) if head else read_worktree(repo, path)

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
            hint_text = found.group(0).lower() if found else "-"
            rows.append((kind, sources[number], hint_text, path, number, text.replace("\t", " ")))

    for entry in touched_entries(repo, merge_base, head):
        old, new = entry.old, entry.new
        old_wanted, new_wanted = wanted(old), wanted(new)
        if not (old_wanted or new_wanted):
            continue
        files.update(p for p, w in ((old, old_wanted), (new, new_wanted)) if w)
        added, removed = changed_lines(repo, entry) if entry.old_regular and entry.new_regular else (None, None)
        if new_wanted and entry.new_regular:
            lines = read_head(new)
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
            out.append(f"| {kind} | {source} | {hint_text} | {location} | {cell} |")
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
    repo = git_text(os.path.abspath(args.repo), "rev-parse", "--show-toplevel").rstrip("\n")
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
