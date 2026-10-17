"""Tests for inventory.py, each run against a real git repository.

Run: python3 -I -B -m unittest discover -s .agents/skills/domain-sweep/scripts
"""

import io
import os
import re
import subprocess
import sys
import sysconfig
import tempfile
import textwrap
import tokenize
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
SCRIPT = os.path.join(HERE, "inventory.py")
sys.path.insert(0, HERE)

import inventory  # noqa: E402

# The oracle tests read the repository's own files and a fixed sample of a
# standard library; ARCLINT_EXHAUSTIVE=1 reads every standard library file.
EXHAUSTIVE = os.environ.get("ARCLINT_EXHAUSTIVE") == "1"
SAMPLE_EVERY = 25


def clean_env():
    """An environment where neither the user's nor a hook's git settings
    reach the test repositories."""
    env = {k: v for k, v in os.environ.items() if not k.startswith("GIT_")}
    env.update(
        GIT_CONFIG_GLOBAL=os.devnull,
        GIT_CONFIG_NOSYSTEM="1",
        GIT_AUTHOR_NAME="t",
        GIT_AUTHOR_EMAIL="t@example.com",
        GIT_COMMITTER_NAME="t",
        GIT_COMMITTER_EMAIL="t@example.com",
    )
    return env


class Repo:
    def __init__(self, root):
        self.root = root
        self.env = clean_env()
        self.git("init", "-q", "-b", "main")

    def git(self, *args):
        result = subprocess.run(
            ["git", "-C", self.root, *args], capture_output=True, text=True, env=self.env,
        )
        if result.returncode != 0:
            raise AssertionError(f"git {' '.join(args)}: {result.stderr}")
        return result.stdout

    def write(self, path, text):
        full = os.path.join(self.root, path)
        os.makedirs(os.path.dirname(full), exist_ok=True)
        with open(full, "w", encoding="utf-8") as handle:
            handle.write(textwrap.dedent(text))

    def remove(self, path):
        os.remove(os.path.join(self.root, path))

    def commit(self, message="c"):
        self.git("add", "-A")
        self.git("commit", "-q", "--allow-empty", "-m", message)
        return self.git("rev-parse", "HEAD").strip()

    def run(self, *args):
        result = subprocess.run(
            [sys.executable, "-I", "-B", SCRIPT, "--repo", self.root, *args],
            capture_output=True, text=True, env=self.env,
        )
        if result.returncode != 0:
            raise AssertionError(f"inventory {' '.join(args)}: {result.stderr}")
        return result

    def rows(self, *args):
        """The rows as (kind, source, hint, location, text) tuples."""
        out = self.run(*args).stdout
        return [tuple(line.split("\t")) for line in out.splitlines()]


class InventoryTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.repo = Repo(self.tmp.name)
        self.repo.write("README.md", "# Project\n")
        self.repo.commit("base")
        self.repo.git("checkout", "-q", "-b", "change")

    def tearDown(self):
        self.tmp.cleanup()

    PRECEDENCE = {"text": 0, "comment": 1, "test": 2}

    def assertRowsInclude(self, rows, required):
        """Every required row is there, with its source or one that ranks
        above it. Where a slash's reading is ambiguous the inventory keeps
        the rows of every reading, so it may list more; it must lose none."""
        sources = {loc: row[1] for loc, row in rows.items()}
        for loc, kind in required.items():
            self.assertIn(loc, sources)
            if kind in self.PRECEDENCE and sources[loc] in self.PRECEDENCE:
                self.assertGreaterEqual(self.PRECEDENCE[sources[loc]], self.PRECEDENCE[kind], loc)
            else:
                self.assertEqual(sources[loc], kind, loc)

    def located(self, rows):
        return {row[3]: row for row in rows}

    def test_a_rule_without_a_keyword_is_read(self):
        # PR #96: the keyword pass missed the paragraph on how the domain
        # file is found, because it states an order and names no keyword.
        self.repo.write("README.md", """\
            # Project

            A project records its domain in the file the ruleset names; when it
            names none, the first of domain.arclint.yaml and domain.arclint.cml
            that exists.
            """)
        rows = self.located(self.repo.rows("--base", "main"))
        for line in (3, 4, 5):
            self.assertEqual(rows[f"README.md:{line}"][:2], ("added", "doc"))
        self.assertEqual(rows["README.md:3"][2], "-")
        self.assertEqual(rows["README.md:4"][4], "names none, the first of domain.arclint.yaml and domain.arclint.cml")

    def test_go_comments_help_text_messages_and_tests(self):
        self.repo.write("cmd/x.go", """\
            package cmd

            import (
            \tfoo "example.com/foo"
            )

            // Resolve finds the recording.
            /* A block comment
               spans lines. */
            const help = `Inspect the domain.

            Every command says so.`

            type T struct {
            \tName string `json:"name,omitempty" yaml:"name"`
            }

            var tick = '`'

            func f() error { return fmt.Errorf("domain file %s lies outside", p) }

            func g() string { return fmt.Sprintf("%s: %v", a, b) }

            func TestRefusesALink(t *testing.T) {}
            """)
        rows = self.located(self.repo.rows("--base", "main"))
        sources = {loc: row[1] for loc, row in rows.items()}
        self.assertEqual(sources.get("cmd/x.go:7"), "comment")
        self.assertEqual(sources.get("cmd/x.go:8"), "comment")
        self.assertEqual(sources.get("cmd/x.go:9"), "comment")
        self.assertEqual(sources.get("cmd/x.go:10"), "text")
        self.assertEqual(sources.get("cmd/x.go:12"), "text")
        self.assertEqual(sources.get("cmd/x.go:20"), "text")
        self.assertEqual(sources.get("cmd/x.go:24"), "test")
        for code in (1, 4, 14, 15, 18, 22):
            self.assertNotIn(f"cmd/x.go:{code}", sources, f"line {code} is code")

    def test_typescript_comments_templates_and_regex(self):
        self.repo.write("web/a.ts", """\
            // Only the host mounts it.
            const re = /\\/\\/ "not a comment/;
            const n = a / b; // divides
            const msg = `The ${kind} ${`nested ${"x"}`} never moves.`;
            const long = `first line
            second line ${value} third`;
            const key = "id";
            test("refuses a file outside the repository", () => {});
            /** Doc comment
             * continues. */
            const done = 1;
            """)
        rows = self.located(self.repo.rows("--base", "main"))
        sources = {loc: row[1] for loc, row in rows.items()}
        self.assertEqual(sources.get("web/a.ts:1"), "comment")
        self.assertNotIn("web/a.ts:2", sources, "a regex literal is code")
        self.assertEqual(sources.get("web/a.ts:3"), "comment")
        self.assertEqual(sources.get("web/a.ts:4"), "text")
        self.assertEqual(sources.get("web/a.ts:5"), "text")
        self.assertEqual(sources.get("web/a.ts:6"), "text")
        self.assertNotIn("web/a.ts:7", sources)
        self.assertEqual(sources.get("web/a.ts:8"), "test")
        self.assertEqual(sources.get("web/a.ts:9"), "comment")
        self.assertEqual(sources.get("web/a.ts:10"), "comment")
        self.assertNotIn("web/a.ts:11", sources)

    def test_python_comments_docstrings_and_tests(self):
        self.repo.write("tools/a.py", '''\
            """A module docstring
            that spans lines."""
            KEY = "id"
            MESSAGE = rb"a file outside the repository"  # never followed
            HASH = "#x"
            def test_refuses_a_link():
                pass
            class TestRecording:
                pass
            value = 1
            ''')
        rows = self.located(self.repo.rows("--base", "main"))
        sources = {loc: row[1] for loc, row in rows.items()}
        self.assertEqual(sources.get("tools/a.py:1"), "text")
        self.assertEqual(sources.get("tools/a.py:2"), "text")
        self.assertNotIn("tools/a.py:3", sources)
        self.assertEqual(sources.get("tools/a.py:4"), "comment")
        self.assertEqual(rows["tools/a.py:4"][2], "never")
        self.assertNotIn("tools/a.py:5", sources, "a hash inside a string is not a comment")
        self.assertEqual(sources.get("tools/a.py:6"), "test")
        self.assertEqual(sources.get("tools/a.py:8"), "test")
        self.assertNotIn("tools/a.py:7", sources)
        self.assertNotIn("tools/a.py:10", sources)

    def test_domain_recordings_are_read_whole(self):
        self.repo.write("domain.arclint.yaml", "contexts:\n  vocabulary:\n    definition: The language.\n")
        self.repo.write("testing/bed/domain.arclint.cml", "BoundedContext Sales {\n}\n")
        self.repo.write("lang/language.yaml", "contexts: {}\n")
        rows = self.located(self.repo.rows("--base", "main", "--recording", "lang/language.yaml"))
        self.assertEqual(rows["domain.arclint.yaml:3"][1], "recording")
        self.assertEqual(rows["testing/bed/domain.arclint.cml:1"][1], "recording")
        self.assertEqual(rows["lang/language.yaml:1"][1], "recording")
        plain = self.located(self.repo.rows("--base", "main"))
        self.assertNotIn("lang/language.yaml:1", plain, "a file is a recording only by its name or --recording")

    def test_removed_lines_and_deleted_files_are_rows_at_their_base_lines(self):
        self.repo.git("checkout", "-q", "main")
        self.repo.write("README.md", "# Project\n\nA Pattern never chooses the file.\nKeep this.\n")
        self.repo.write("docs/old.md", "Old guarantee.\n")
        self.repo.commit("more")
        self.repo.git("checkout", "-q", "change")
        self.repo.git("reset", "-q", "--hard", "main")
        self.repo.write("README.md", "# Project\n\nKeep this.\n")
        self.repo.remove("docs/old.md")
        rows = self.located(self.repo.rows("--base", "main", "--scope", "changed"))
        self.assertEqual(rows["README.md:3"][:3], ("deleted", "doc", "never"))
        self.assertEqual(rows["docs/old.md:1"][:2], ("deleted", "doc"))
        self.assertNotIn("README.md:4", rows, "scope changed lists no unchanged line")
        files = self.repo.run("--base", "main", "--files").stdout.split()
        self.assertEqual(files, ["README.md", "docs/old.md"])

    def test_unchanged_lines_of_touched_files_under_scope_touched(self):
        self.repo.write("README.md", "# Project\n\nNew line.\n")
        rows = self.located(self.repo.rows("--base", "main"))
        self.assertEqual(rows["README.md:1"][0], "existing")
        self.assertEqual(rows["README.md:3"][0], "added")

    def test_quoted_and_unicode_paths(self):
        self.repo.write('docs/"quoted".md', "A quoted name.\n")
        self.repo.write("docs/résumé.md", "An accented name.\n")
        self.repo.commit("names")
        rows = self.located(self.repo.rows("--base", "main", "--head", "change"))
        self.assertIn('docs/"quoted".md:1', rows)
        self.assertIn("docs/résumé.md:1", rows)

    def test_content_lines_that_look_like_headers(self):
        # Inside a hunk, a removed "-- x" line reads "--- x" and an added
        # "++ y" line reads "+++ y"; neither names a file.
        self.repo.git("checkout", "-q", "main")
        self.repo.write("notes.md", "intro\n-- x\n")
        self.repo.commit("notes")
        self.repo.git("checkout", "-q", "change")
        self.repo.git("reset", "-q", "--hard", "main")
        self.repo.write("notes.md", "intro\n++ y\n")
        rows = self.repo.rows("--base", "main", "--scope", "changed")
        self.assertEqual(
            [(row[0], row[3], row[4]) for row in rows],
            [("added", "notes.md:2", "++ y"), ("deleted", "notes.md:2", "-- x")],
        )

    def test_untracked_files_are_read_and_ignored_ones_are_not(self):
        self.repo.write(".gitignore", "build/\n")
        self.repo.commit("ignore")
        self.repo.write("NEW.md", "Untracked rule.\n")
        self.repo.write("build/out.md", "Ignored.\n")
        rows = self.located(self.repo.rows("--base", "main"))
        self.assertIn("NEW.md:1", rows)
        self.assertNotIn("build/out.md:1", rows)

    def test_head_reads_the_commit_not_the_worktree(self):
        self.repo.write("README.md", "# Project\n\nCommitted.\n")
        head = self.repo.commit("head")
        self.repo.write("README.md", "# Project\n\nWorking tree.\n")
        self.repo.write("UNTRACKED.md", "Untracked.\n")
        rows = self.located(self.repo.rows("--base", "main", "--head", head))
        self.assertEqual(rows["README.md:3"][4], "Committed.")
        self.assertNotIn("UNTRACKED.md:1", rows)

    def test_the_change_starts_at_the_merge_base(self):
        self.repo.write("BRANCH.md", "Branch work.\n")
        self.repo.commit("branch")
        self.repo.git("checkout", "-q", "main")
        self.repo.write("MAIN.md", "Main moved on.\n")
        self.repo.commit("main")
        self.repo.git("checkout", "-q", "change")
        rows = self.located(self.repo.rows("--base", "main"))
        self.assertIn("BRANCH.md:1", rows)
        self.assertNotIn("MAIN.md:1", rows, "a commit on the base alone is not part of the change")

    def test_generated_files_have_no_rows_but_are_listed(self):
        self.repo.write("gen.go", "// Code generated by tools/gen; DO NOT EDIT.\n\npackage gen\n\n// A comment.\n")
        self.repo.write("docs/page.md", '+++\ntitle = "x"\n+++\n\n<!-- Code generated by tools/x; DO NOT EDIT. -->\n\nText.\n')
        self.repo.write("hand.go", "package hand\n\n// Mentions Code generated text and DO NOT EDIT later.\n")
        result = self.repo.run("--base", "main")
        rows = self.located([tuple(line.split("\t")) for line in result.stdout.splitlines()])
        self.assertNotIn("gen.go:5", rows)
        self.assertNotIn("docs/page.md:7", rows)
        self.assertIn("hand.go:3", rows)
        self.assertIn("generated files have no rows: docs/page.md, gen.go", result.stderr)
        files = self.repo.run("--base", "main", "--files").stdout.split()
        self.assertIn("gen.go", files)

    def test_claude_skill_copies_are_excluded(self):
        self.repo.write(".agents/skills/s/SKILL.md", "Source.\n")
        self.repo.write(".claude/skills/s/SKILL.md", "Source.\n")
        self.repo.write("bed/.claude/skills/s/SKILL.md", "Source.\n")
        rows = self.located(self.repo.rows("--base", "main"))
        self.assertIn(".agents/skills/s/SKILL.md:1", rows)
        self.assertNotIn(".claude/skills/s/SKILL.md:1", rows)
        self.assertNotIn("bed/.claude/skills/s/SKILL.md:1", rows)

    def test_fixtures_and_exports_are_excluded(self):
        self.repo.write("pkg/testdata/docs/vendored.md", "Vendored text.\n")
        self.repo.write("domain.arclint.cml", "BoundedContext A {\n}\n")
        self.repo.write("bed/domain.arclint.cml", "BoundedContext B {\n}\n")
        rows = self.located(self.repo.rows("--base", "main"))
        self.assertNotIn("pkg/testdata/docs/vendored.md:1", rows)
        self.assertNotIn("domain.arclint.cml:1", rows, "the root CML file is an export")
        self.assertIn("bed/domain.arclint.cml:1", rows, "a CML recording elsewhere is read")

    def test_test_files_list_comments_and_names_not_fixtures(self):
        self.repo.write("pkg/a_test.go", """\
            package pkg

            // A refused link leaves the file untouched.
            func TestRefusesALink(t *testing.T) {
            \tfixture := `rules:
              domain: outside the repository`
            \tt.Errorf("the file changed after refusal")
            }
            """)
        self.repo.write("web/a.test.ts", 'it("keeps the mount", () => { const s = "two words"; });\n')
        self.repo.write("tools/test_a.py", 'MESSAGE = "two words"  # a comment\n')
        rows = self.located(self.repo.rows("--base", "main"))
        sources = {loc: row[1] for loc, row in rows.items()}
        self.assertEqual(sources.get("pkg/a_test.go:3"), "comment")
        self.assertEqual(sources.get("pkg/a_test.go:4"), "test")
        for line in (5, 6, 7):
            self.assertNotIn(f"pkg/a_test.go:{line}", sources)
        self.assertEqual(sources.get("web/a.test.ts:1"), "test")
        self.assertEqual(sources.get("tools/test_a.py:1"), "comment")

    def test_a_name_with_a_space_is_read(self):
        # Git ends a ---/+++ header path holding a space with a tab.
        self.repo.write("docs/with space.md", "A rule.\n")
        self.repo.commit("space")
        rows = self.located(self.repo.rows("--base", "main", "--head", "HEAD"))
        self.assertIn("docs/with space.md:1", rows)
        files = self.repo.run("--base", "main", "--head", "HEAD", "--files").stdout.split("\n")
        self.assertIn("docs/with space.md", files)

    def test_quoted_names_keep_their_characters(self):
        for name in ('docs/\u6587".md', 'docs/r\u00e9sum\u00e9".md', "docs/tab\there.md"):
            self.repo.write(name, "A rule.\n")
        self.repo.commit("names")
        rows = self.located(self.repo.rows("--base", "main", "--head", "HEAD"))
        self.assertIn('docs/\u6587".md:1', rows)
        self.assertIn('docs/r\u00e9sum\u00e9".md:1', rows)
        self.assertIn('"docs/tab\\there.md":1', rows, "a control character is escaped, not a column break")

    def test_joined_hunks_keep_their_line_numbers(self):
        self.repo.git("checkout", "-q", "main")
        self.repo.write("notes.md", "one\nkeep a\nkeep b\nkeep c\nfive\n")
        self.repo.commit("notes")
        self.repo.git("checkout", "-q", "change")
        self.repo.git("reset", "-q", "--hard", "main")
        self.repo.git("config", "diff.interHunkContext", "3")
        self.repo.write("notes.md", "ONE\nkeep a\nkeep b\nkeep c\nFIVE\n")
        rows = self.repo.rows("--base", "main", "--scope", "changed")
        self.assertEqual(
            sorted((row[0], row[3], row[4]) for row in rows),
            [("added", "notes.md:1", "ONE"), ("added", "notes.md:5", "FIVE"),
             ("deleted", "notes.md:1", "one"), ("deleted", "notes.md:5", "five")],
        )

    def test_text_conversion_does_not_move_lines(self):
        self.repo.git("checkout", "-q", "main")
        self.repo.write(".gitattributes", "*.md diff=words\n")
        self.repo.write("rules.md", "Header\nFirst rule.\nSecond rule.\n")
        self.repo.commit("rules")
        self.repo.git("checkout", "-q", "change")
        self.repo.git("reset", "-q", "--hard", "main")
        self.repo.git("config", "diff.words.textconv", "sed 1d")
        self.repo.write("rules.md", "Header\nFirst rule.\nSecond rule, changed.\n")
        rows = self.repo.rows("--base", "main", "--scope", "changed")
        self.assertEqual(
            sorted((row[0], row[3]) for row in rows),
            [("added", "rules.md:3"), ("deleted", "rules.md:3")],
        )

    def test_a_removal_alone_still_lists_the_file_s_other_lines(self):
        self.repo.git("checkout", "-q", "main")
        self.repo.write("rules.md", "Kept statement.\nRemoved statement.\n")
        self.repo.commit("rules")
        self.repo.git("checkout", "-q", "change")
        self.repo.git("reset", "-q", "--hard", "main")
        self.repo.write("rules.md", "Kept statement.\n")
        rows = self.repo.rows("--base", "main")
        self.assertEqual(
            sorted((row[0], row[3]) for row in rows),
            [("deleted", "rules.md:2"), ("existing", "rules.md:1")],
        )

    def test_a_pure_rename_is_touched(self):
        self.repo.git("checkout", "-q", "main")
        self.repo.write("old.go", "package p\n\n// A rule the file states.\n")
        self.repo.commit("old")
        self.repo.git("checkout", "-q", "change")
        self.repo.git("reset", "-q", "--hard", "main")
        self.repo.git("mv", "old.go", "new.go")
        files = self.repo.run("--base", "main", "--files").stdout.split()
        self.assertEqual(files, ["new.go", "old.go"])
        rows = self.repo.rows("--base", "main")
        self.assertEqual([(row[0], row[3]) for row in rows], [("existing", "new.go:3")])

    def test_a_regex_after_a_condition_is_not_a_comment(self):
        self.repo.write("web/a.ts", 'if (true) /[/*]/.test("/");\nconst n = 1;\nconst m = (a) / 2; // half\n')
        rows = self.located(self.repo.rows("--base", "main"))
        self.assertNotIn("web/a.ts:1", rows)
        self.assertNotIn("web/a.ts:2", rows)
        self.assertEqual(rows["web/a.ts:3"][1], "comment")

    def test_a_typescript_test_name_on_the_next_line(self):
        self.repo.write("web/x.test.ts", 'test(\n  "refuses an outside path",\n  () => {});\n')
        rows = self.located(self.repo.rows("--base", "main"))
        self.assertEqual(rows["web/x.test.ts:1"][1], "test")
        self.assertEqual(rows["web/x.test.ts:2"][1], "test")
        self.assertNotIn("web/x.test.ts:3", rows)

    def test_declarations_inside_fixtures_are_not_tests(self):
        self.repo.write("pkg/x_test.go", "package pkg\n\nvar fixture = `\nfunc TestNotReal(t *testing.T) {}\n`\n")
        self.repo.write("web/x.test.ts", 'const f = `\ntest("not a real test", () => {});\n`;\n')
        self.repo.write("tools/test_x.py", 'F = """\ndef test_not_real():\n"""\n')
        rows = self.located(self.repo.rows("--base", "main"))
        self.assertNotIn("pkg/x_test.go:4", rows)
        self.assertNotIn("web/x.test.ts:2", rows)
        self.assertNotIn("tools/test_x.py:2", rows)

    def test_a_string_continued_past_a_line_is_text(self):
        self.repo.write("tools/a.py", 'message = "first part \\\nnever overwrite"\nvalue = 1\n')
        self.repo.write("web/a.ts", 'const message = "first part \\\nnever overwrite";\nconst value = 1;\n')
        rows = self.located(self.repo.rows("--base", "main"))
        for path in ("tools/a.py", "web/a.ts"):
            self.assertEqual(rows[f"{path}:1"][1], "text")
            self.assertEqual(rows[f"{path}:2"][1], "text")
            self.assertNotIn(f"{path}:3", rows)

    def test_a_head_ref_that_is_also_a_path(self):
        self.repo.write("change/README.md", "A rule.\n")
        self.repo.commit("dir named like the branch")
        rows = self.located(self.repo.rows("--base", "main", "--head", "change"))
        self.assertIn("change/README.md:1", rows)

    def test_an_unknown_ref_is_named(self):
        result = subprocess.run(
            [sys.executable, "-I", "-B", SCRIPT, "--repo", self.repo.root, "--base", "nowhere"],
            capture_output=True, text=True, env=self.repo.env,
        )
        self.assertEqual(result.returncode, 1)
        self.assertIn("nowhere names no commit", result.stderr)

    def test_run_from_a_subdirectory(self):
        self.repo.git("checkout", "-q", "main")
        self.repo.write("pkg/note.md", "Before.\n")
        self.repo.commit("note")
        self.repo.git("checkout", "-q", "change")
        self.repo.git("reset", "-q", "--hard", "main")
        self.repo.write("pkg/note.md", "After.\n")
        result = subprocess.run(
            [sys.executable, "-I", "-B", SCRIPT, "--repo", os.path.join(self.repo.root, "pkg"),
             "--base", "main", "--scope", "changed"],
            capture_output=True, text=True, env=self.repo.env,
        )
        self.assertEqual(sorted(line.split("\t")[3] for line in result.stdout.splitlines()),
                         ["pkg/note.md:1", "pkg/note.md:1"])

    def test_a_text_file_marked_binary_still_has_hunks(self):
        self.repo.git("checkout", "-q", "main")
        self.repo.write(".gitattributes", "*.md -diff\n")
        self.repo.write("note.md", "Before.\nKept.\n")
        self.repo.commit("note")
        self.repo.git("checkout", "-q", "change")
        self.repo.git("reset", "-q", "--hard", "main")
        self.repo.write("note.md", "After.\nKept.\n")
        rows = self.repo.rows("--base", "main")
        self.assertEqual(
            sorted((row[0], row[3]) for row in rows),
            [("added", "note.md:1"), ("deleted", "note.md:1"), ("existing", "note.md:2")],
        )

    def test_a_slash_after_a_postfix_operator_divides(self):
        self.repo.write("web/a.ts", "let n = 2; const half = n++ / 2; // Only the owner writes.\nconst m = n-- / 2; // Never twice.\nconst r = ++n;\n")
        rows = self.located(self.repo.rows("--base", "main"))
        self.assertEqual(rows["web/a.ts:1"][1], "comment")
        self.assertEqual(rows["web/a.ts:2"][1], "comment")
        self.assertNotIn("web/a.ts:3", rows)

    def test_a_test_name_after_a_blank_or_comment_line(self):
        self.repo.write("web/x.test.ts", 'test(\n\n  // why\n  "never overwrites an existing file", () => {});\n')
        rows = self.located(self.repo.rows("--base", "main"))
        self.assertEqual(rows["web/x.test.ts:4"][1], "test")

    def test_lines_split_where_git_splits_them(self):
        self.repo.write("a.go", 'package p\n\nvar s = "before\u2028after\x0cform"\n')
        self.repo.commit("a")
        self.repo.write("a.go", 'package p\n\nvar s = "before\u2028after\x0cform"\n// Only the owner writes.\n')
        rows = self.located(self.repo.rows("--base", "main", "--scope", "changed"))
        self.assertEqual(rows["a.go:4"][:2], ("added", "comment"))

    def test_a_licence_block_does_not_hide_the_generated_marker(self):
        self.repo.write("gen.go", "/*\nCopyright 2026.\n*/\n// Code generated by gen; DO NOT EDIT.\npackage p\n")
        self.repo.write("gen.md", "<!--\nLicence.\n-->\n<!-- Code generated by x; DO NOT EDIT. -->\n\nText.\n")
        result = self.repo.run("--base", "main")
        self.assertEqual(result.stdout, "")
        self.assertIn("generated files have no rows: gen.go, gen.md", result.stderr)

    def test_markdown_output_escapes_the_location(self):
        self.repo.write("docs/a|b.md", "A rule.\n")
        out = self.repo.run("--base", "main", "--format", "md").stdout.splitlines()
        self.assertEqual(out[2], "| added | doc | - | docs/a\\|b.md:1 | A rule. |")

    def test_a_lone_carriage_return_stays_in_its_line(self):
        self.repo.git("checkout", "-q", "main")
        with open(os.path.join(self.repo.root, "README.md"), "wb") as handle:
            handle.write(b"First\rparagraph\nOld rule.\n")
        self.repo.commit("cr")
        self.repo.git("checkout", "-q", "change")
        self.repo.git("reset", "-q", "--hard", "main")
        with open(os.path.join(self.repo.root, "README.md"), "wb") as handle:
            handle.write(b"First\rparagraph\nNew rule.\n")
        rows = self.repo.rows("--base", "main", "--scope", "changed")
        self.assertEqual(sorted((row[0], row[3], row[4]) for row in rows),
                         [("added", "README.md:2", "New rule."), ("deleted", "README.md:2", "Old rule.")])

    def test_a_non_null_assertion_before_a_division(self):
        self.repo.write("web/a.ts", "const count: number | undefined = 4;\nconst n = count! / 2; // Only the owner writes.\nconst b = !/x/.test(s);\n")
        rows = self.located(self.repo.rows("--base", "main"))
        self.assertEqual(rows["web/a.ts:2"][1], "comment")
        self.assertNotIn("web/a.ts:3", rows)

    def test_a_test_name_template_spanning_lines(self):
        self.repo.write("web/x.test.ts", "test(`never\noverwrites existing files`, () => {});\nconst x = 1;\n")
        rows = self.located(self.repo.rows("--base", "main"))
        self.assertEqual(rows["web/x.test.ts:1"][1], "test")
        self.assertEqual(rows["web/x.test.ts:2"][1], "test")
        self.assertNotIn("web/x.test.ts:3", rows)

    def test_a_condition_keyword_on_the_line_before(self):
        self.repo.write("web/a.ts", 'if\n(true) /[/*]/.test("/");\nconst value = 1;\n')
        rows = self.located(self.repo.rows("--base", "main"))
        self.assertEqual(rows, {})

    def test_a_committed_link_is_not_read_as_its_target(self):
        self.repo.write("docs/source.md", "Source.\n")
        self.repo.commit("source")
        os.symlink("docs/source.md", os.path.join(self.repo.root, "README2.md"))
        self.repo.commit("link")
        rows = self.located(self.repo.rows("--base", "main", "--head", "HEAD"))
        self.assertNotIn("README2.md:1", rows)
        self.assertIn("docs/source.md:1", rows)

    def test_a_repository_path_ending_in_a_space(self):
        root = os.path.join(self.tmp.name, "nested ")
        os.mkdir(root)
        repo = Repo(root)
        repo.write("README.md", "Base.\n")
        repo.commit("base")
        repo.write("README.md", "Base.\nNew rule.\n")
        rows = repo.rows("--base", "main", "--scope", "changed")
        self.assertEqual([row[3] for row in rows], ["README.md:2"])

    def test_a_regex_after_a_keyword_on_the_line_before(self):
        self.repo.write("web/a.ts", 'async function f() {\n  return await\n  /[`]/.test("");\n}\n// Never overwrite.\n')
        rows = self.located(self.repo.rows("--base", "main"))
        self.assertRowsInclude(rows, {"web/a.ts:5": "comment"})

    def test_a_comment_between_a_condition_and_its_parenthesis(self):
        self.repo.write("web/a.ts", 'if /* ready */ (true) /[`]/.test("");\n// Never overwrite.\n')
        rows = self.located(self.repo.rows("--base", "main"))
        self.assertEqual({loc: row[1] for loc, row in rows.items()},
                         {"web/a.ts:1": "comment", "web/a.ts:2": "comment"})

    def test_a_parameterized_test_name(self):
        self.repo.write("web/a.test.ts", 'test.each([1, 2])("rejects invalid input %s", () => {});\n'
                        'describe.each([[1, f(2)]])("never %s", () => {});\n')
        rows = self.located(self.repo.rows("--base", "main"))
        self.assertEqual(rows["web/a.test.ts:1"][1], "test")
        self.assertEqual(rows["web/a.test.ts:2"][1], "test")

    def test_a_non_null_assertion_after_a_tagged_template(self):
        self.repo.write("web/a.ts", "const half = count`x`! / 2; // Only the owner writes.\nconst q = a[0] / b / c; // Divides twice.\n")
        rows = self.located(self.repo.rows("--base", "main"))
        self.assertEqual(rows["web/a.ts:1"][1], "comment")
        self.assertEqual(rows["web/a.ts:2"][1], "comment")

    def test_test_names_after_tables_and_type_arguments(self):
        self.repo.write("web/a.test.ts", textwrap.dedent("""\
            test.each([
              [1, 2],
            ])("never %s twice", () => {});
            test.each`
              a | b
            `("refuses $a", () => {});
            it.each<Row>(["mac", "win"])("binds on %s", () => {});
            test("closes", () => { const f = `
            fixture`; });
            test(`spans ${kind}
            lines`, () => {});
            """))
        rows = self.located(self.repo.rows("--base", "main"))
        sources = {loc: row[1] for loc, row in rows.items()}
        self.assertEqual(sources, {
            "web/a.test.ts:1": "test", "web/a.test.ts:3": "test",
            "web/a.test.ts:4": "test", "web/a.test.ts:6": "test",
            "web/a.test.ts:7": "test", "web/a.test.ts:8": "test",
            "web/a.test.ts:10": "test", "web/a.test.ts:11": "test",
        })

    def test_keyword_properties_and_for_await(self):
        self.repo.write("web/a.ts", textwrap.dedent("""\
            const ratio = counts.in / 2; // Never round.
            async function f(xs: AsyncIterable<string>) { for await (const x of xs) /[`]/.test(x); }
            // Only the owner writes.
            """))
        rows = self.located(self.repo.rows("--base", "main"))
        self.assertEqual({loc: row[1] for loc, row in rows.items()},
                         {"web/a.ts:1": "comment", "web/a.ts:3": "comment"})

    def test_comments_inside_f_string_fields(self):
        self.repo.write("tools/test_payload.py", textwrap.dedent('''\
            payload = f"""{
                value  # never logged
            } and {other!r:>10}"""
            message = f"{a["key"]} two words"
            '''))
        rows = self.located(self.repo.rows("--base", "main"))
        self.assertEqual({loc: row[1] for loc, row in rows.items()}, {"tools/test_payload.py:2": "comment"})
        self.repo.write("tools/payload.py", 'f"{x} spans two {y}"\nf"{x}{y}"\n')
        rows = self.located(self.repo.rows("--base", "main"))
        self.assertEqual(rows["tools/payload.py:1"][1], "text")
        self.assertNotIn("tools/payload.py:2", rows)

    def test_round_six_lexer_cases(self):
        self.repo.write("web/callbacks.test.ts", 'test.each<[() => void]>([[() => {}]])("never writes twice", (write) => {});\n')
        self.repo.write("web/debug.ts", 'debugger\n/[`]/.test("x");\n// Never overwrite.\n')
        self.repo.write("web/counts.ts", "class Counts { #in = 4; half() { return this.#in / 2; /* Never round. */ } }\n")
        self.repo.write("web/cli.ts", "#!/usr/bin/env node\n// Only the owner runs it.\n")
        self.repo.write("pkg/split_test.go", 'package p\n\nimport "testing"\n\nfunc\nTestNeverOverwrites(t *testing.T) {}\n')
        self.repo.write("tools/test_payload.py", 'payload = rf"""\\{(\n    1 + 2  # Never log the secret.\n)}"""\n')
        self.repo.write("tools/joined.py", "def \\\n    test_joined():\n    pass\n")
        self.repo.write("tools/named.py", 'x = f"\\N{EM DASH} {value} two words"\n')
        self.repo.write("guide.md", "# Code generated files: DO NOT EDIT\n\nAlways run the generator.\n")
        rows = self.located(self.repo.rows("--base", "main"))
        sources = {loc: row[1] for loc, row in rows.items()}
        self.assertEqual(sources.get("web/callbacks.test.ts:1"), "test")
        self.assertEqual(sources.get("web/debug.ts:3"), "comment")
        self.assertEqual(sources.get("web/counts.ts:1"), "comment")
        self.assertNotIn("web/cli.ts:1", sources)
        self.assertEqual(sources.get("web/cli.ts:2"), "comment")
        self.assertEqual(sources.get("pkg/split_test.go:6"), "test")
        self.assertEqual(sources.get("tools/test_payload.py:2"), "comment")
        self.assertEqual(sources.get("tools/joined.py:1"), "test")
        self.assertEqual(sources.get("tools/named.py:1"), "text")
        self.assertEqual(sources.get("guide.md:3"), "doc", "a Markdown heading declares nothing generated")

    def test_round_seven_lexer_cases(self):
        self.repo.write("web/x.ts", textwrap.dedent("""\
            const half = +{ valueOf: () => 4 } / 2; // Never round.
            const of = 4; const h = of / 2; // Only once.
            loop: while (true) { break loop
            /[/*]/.test(""); }
            const value = 1;
            for (const x of /[`]/.exec(s) ?? []) {}
            switch (k) { case 1: { } /[`]/.test(s); }
            if (ok) { } /[`]/.test(s);
            // The end.
            """))
        self.repo.write("gen/line.go", "package p\n//line template.tmpl:100\n// Never overwrite.\n")
        rows = self.located(self.repo.rows("--base", "main"))
        self.assertRowsInclude(rows, {
            "web/x.ts:1": "comment", "web/x.ts:2": "comment", "web/x.ts:9": "comment",
        })
        self.assertEqual(rows["gen/line.go:3"][1], "comment")

    def test_round_eight_lexer_cases(self):
        self.repo.write("web/check.ts", 'setup()\n!/[`]/.test(value) && fail();\n// Never allow a backtick.\n')
        self.repo.write("web/label.ts", 'done: { break done; }\n/[`]/.test("");\n// Never overwrite.\n')
        self.repo.write("web/expr.ts", "const half = function() {} / 2; // Never round.\n")
        self.repo.write("web/loop.ts", "const of = 4; for (let i = of / 2; i > 0; i--) { /* Never round. */ }\n"
                        "for (const of of [1]) { /* Only once. */ }\n"
                        "for (const {a} of xs) { /* Each. */ }\n")
        self.repo.write("web/obj.ts", "const o = { a: { b: 1 } / 2 }; // Not a label.\n")
        rows = self.located(self.repo.rows("--base", "main"))
        self.assertRowsInclude(rows, {
            "web/check.ts:3": "comment", "web/label.ts:3": "comment", "web/expr.ts:1": "comment",
            "web/loop.ts:1": "comment", "web/loop.ts:2": "comment", "web/loop.ts:3": "comment",
            "web/obj.ts:1": "comment",
        })

    def test_round_nine_lexer_cases(self):
        self.repo.write("web/x.test.ts", 'export default function setup() {}\n/[`]/.test("x");\n// Never overwrite.\n')
        self.repo.write("web/script.ts", "const await = 4;\nconst half = await / 2; // Never round.\n"
                        "function f(yield) { return yield / 2; /* Only half. */ }\n"
                        "const g = async function () {} / 2; // Not a number.\n")
        self.repo.write("tools/test_identity.py", "def test_identity[T: (int, str)](value: T):\n    assert value == value\n")
        rows = self.located(self.repo.rows("--base", "main"))
        self.assertEqual({loc: row[1] for loc, row in rows.items()}, {
            "web/x.test.ts:3": "comment", "web/script.ts:2": "comment", "web/script.ts:3": "comment",
            "web/script.ts:4": "comment", "tools/test_identity.py:1": "test",
        })

    def test_round_ten_cases(self):
        self.repo.write("web/a.test.ts", textwrap.dedent("""\
            type Box<T> = number; const value = 4; const half = value as Box<number> / 2; // Never round.
            function identity(await) { return await; }
            async function check() { return await /[`]/.test("x"); }
            // Never overwrite.
            const h = (yield) => yield / 2; // Only half.
            const f = +function(n = {}) { return 4; } / 2; // Not a number.
            """))
        self.repo.write("tools/test_identity.py", textwrap.dedent("""\
            def test_deep[T: tuple[list[tuple[int, str]], str]](): pass
            def test_split[
                T,
            ](): pass
            """))
        self.repo.write("pkg/x_test.go", "package p\n\nimport \"testing\"\n\nfunc /* ownership */ TestNeverOverwrites(t *testing.T) {}\n")
        rows = self.located(self.repo.rows("--base", "main"))
        self.assertEqual({loc: row[1] for loc, row in rows.items()}, {
            "web/a.test.ts:1": "comment", "web/a.test.ts:4": "comment", "web/a.test.ts:5": "comment",
            "web/a.test.ts:6": "comment", "tools/test_identity.py:1": "test", "tools/test_identity.py:2": "test",
            "pkg/x_test.go:5": "test",
        })

    def test_a_reused_path_keeps_its_own_hunks(self):
        self.repo.git("checkout", "-q", "main")
        self.repo.write("old.md", "Old rule.\n")
        self.repo.commit("old")
        self.repo.git("checkout", "-q", "change")
        self.repo.git("reset", "-q", "--hard", "main")
        self.repo.git("mv", "old.md", "new.md")
        self.repo.write("old.md/replacement.md", "New rule.\n")
        self.repo.commit("rename and reuse")
        rows = self.repo.rows("--base", "main", "--head", "HEAD", "--scope", "changed")
        self.assertEqual(sorted((row[0], row[3]) for row in rows), [("added", "old.md/replacement.md:1")])
        rows = self.repo.rows("--base", "main", "--scope", "changed")
        self.assertEqual(sorted((row[0], row[3]) for row in rows), [("added", "old.md/replacement.md:1")])

    def test_round_eleven_cases(self):
        self.repo.write("web/assertion.ts", 'type Box<T> = T; const ratio = 4 as Box<number> / Math.max("never round".length, 1) / 2;\n')
        self.repo.write("web/binding.ts", 'function identity(await: number) { return await; }\n'
                        'async function check() { return await /[/*]/.test("x"); }\nconst n = 1;\n// Never overwrite.\n')
        self.repo.write("web/defaults.ts", 'const half = +function(n = {}) { return 4; } / Math.max("never round".length, 1) / 2;\n')
        self.repo.write("web/alias.ts", 'type Props = { value: string }\n/[/*]/.test("x");\nconst n = 1;\n// Never overwrite.\n')
        rows = self.located(self.repo.rows("--base", "main"))
        self.assertRowsInclude(rows, {
            "web/assertion.ts:1": "text", "web/binding.ts:4": "comment",
            "web/defaults.ts:1": "text", "web/alias.ts:4": "comment",
        })

    def test_round_twelve_cases(self):
        self.repo.write("handwritten.go", '// Recognize "Code generated ... DO NOT EDIT" headers.\npackage p\n// Never overwrite handwritten files.\n')
        self.repo.write("web/qualified.ts", "namespace Units { export type Count<T> = number; }\n"
                        "const ratio = 4 as Units.Count<number> / `never overwrite`.length / 2;\n")
        self.repo.write("pkg/inline_test.go", 'package p\n\nimport "testing"\n\nfunc helper() {}; func TestNeverOverwrites(t *testing.T) {}\n')
        self.repo.write("tools/message.py", 'message = f"{1:\\N{EM DASH}>10}"\n')
        result = self.repo.run("--base", "main")
        rows = self.located([tuple(line.split("\t")) for line in result.stdout.splitlines()])
        self.assertNotIn("generated files", result.stderr)
        self.assertEqual({loc: row[1] for loc, row in rows.items()}, {
            "handwritten.go:1": "comment", "handwritten.go:3": "comment", "web/qualified.ts:2": "text",
            "pkg/inline_test.go:5": "test", "tools/message.py:1": "text",
        })

    def test_round_thirteen_cases(self):
        self.repo.write("web/suite.test.ts", textwrap.dedent("""\
            const suiteName = "recording"; describe(suiteName, () => { test("never overwrites", () => {}); });
            describe(Thing.name, () => {
              describe(other, () => {
                it("keeps the file", () => {});
              });
              test(`refuses ${x}`, () => {});
            });
            """))
        self.repo.write("web/ratio.ts", "const ratio = 4 satisfies { valueOf(): number } / `never round`.length / 2;\n")
        self.repo.write("web/generated.ts", "#!/usr/bin/env node\n// Code generated by tools/gen; DO NOT EDIT.\n// Never edit this file.\n")
        result = self.repo.run("--base", "main")
        rows = self.located([tuple(line.split("\t")) for line in result.stdout.splitlines()])
        self.assertEqual({loc: row[1] for loc, row in rows.items()}, {
            "web/suite.test.ts:1": "test", "web/suite.test.ts:4": "test", "web/suite.test.ts:6": "test",
            "web/ratio.ts:1": "text",
        })
        self.assertIn("web/generated.ts", result.stderr)

    def test_round_fourteen_cases(self):
        self.repo.write("web/hand.ts", "/* Licence. */ const help = `\n// Code generated by example; DO NOT EDIT.\n"
                        "Never overwrite existing files.\n`;\n")
        self.repo.write("web/ratio.ts", "const ratio = +function(): { value: number } { return { value: 4 }; } / `never round`.length / 2;\n"
                        "const half = +function(): Promise<void> { return p; } / 2; // Only half.\n")
        with open(os.path.join(self.repo.root, "test_bom.py"), "wb") as handle:
            handle.write(b"\xef\xbb\xbfdef test_never_overwrites():\n    pass\n")
        result = self.repo.run("--base", "main")
        rows = self.located([tuple(line.split("\t")) for line in result.stdout.splitlines()])
        self.assertNotIn("generated", result.stderr)
        self.assertEqual({loc: row[1] for loc, row in rows.items()}, {
            "web/hand.ts:1": "comment", "web/hand.ts:2": "text", "web/hand.ts:3": "text", "web/hand.ts:4": "text",
            "web/ratio.ts:1": "text", "web/ratio.ts:2": "comment", "test_bom.py:1": "test",
        })

    def test_round_fifteen_cases(self):
        self.repo.write("web/sample.test.ts", "/* Licence. */ /* eslint-disable */ const sample = `\n"
                        "// Code generated by example; DO NOT EDIT.\n`;\n"
                        'test("never overwrites user files", () => {});\n')
        self.repo.write("web/ratio.ts", 'const ratio = 4 as import("./units").Count<number> / "say \\"never overwrite\\"".length / 2;\n')
        self.repo.write("web/suite.test.ts", 'test.each([\n  { test: 1 },\n])("refuses", () => {});\n')
        result = self.repo.run("--base", "main")
        rows = self.located([tuple(line.split("\t")) for line in result.stdout.splitlines()])
        self.assertNotIn("generated", result.stderr)
        self.assertEqual({loc: row[1] for loc, row in rows.items()}, {
            "web/sample.test.ts:1": "comment", "web/sample.test.ts:4": "test",
            "web/ratio.ts:1": "text", "web/suite.test.ts:1": "test", "web/suite.test.ts:3": "test",
        })

    def test_a_misread_slash_that_opens_a_comment(self):
        self.repo.write("web/x.test.ts", 'type Unit = number\n/[/*]/.test(source);\nconst end = `*/`;\n'
                        '// Never overwrite.\ntest("rejects", () => {});\nconst q = a / b; /* why\n  it halves */\n')
        rows = self.located(self.repo.rows("--base", "main"))
        self.assertRowsInclude(rows, {
            "web/x.test.ts:4": "comment", "web/x.test.ts:5": "test",
            "web/x.test.ts:6": "comment", "web/x.test.ts:7": "comment",
        })

    def test_round_seventeen_cases(self):
        self.repo.write("web/x.test.ts", 'type Box<T> = T;\nconst ratio = 4 as number | Box<number> / 2; test("never overwrite", () => {}); const half = 4 / 2;\n')
        rows = self.located(self.repo.rows("--base", "main"))
        self.assertEqual({loc: row[1] for loc, row in rows.items()}, {"web/x.test.ts:2": "test"})
        root = os.path.join(os.fsencode(self.tmp.name), b"repo\xff")
        os.mkdir(root)
        repo = Repo(os.fsdecode(root))
        repo.write("README.md", "Base.\n")
        repo.commit("base")
        repo.write("NEW.md", "A rule.\n")
        rows = repo.rows("--base", "main", "--scope", "changed")
        self.assertEqual([row[3] for row in rows], ["NEW.md:1"])

    def test_round_eighteen_cases(self):
        self.repo.write("web/sample.test.ts", 'const ratio = 4 as import("./units").Count<number> / (test("rejects", () => {}), 2) / 2;\n'
                        "const cafe\u0301 = 4; const h = cafe\u0301 / 2; // Never round.\n")
        self.repo.write("test_rules.py", "def test_cafe\u0301():\n    pass\n")
        with open(os.path.join(self.repo.root, "p.go"), "wb") as handle:
            handle.write(b"package p\nvar help = `never\r overwrite`\n")
        rows = self.located(self.repo.rows("--base", "main"))
        self.assertEqual({loc: row[1] for loc, row in rows.items()}, {
            "web/sample.test.ts:1": "test", "web/sample.test.ts:2": "comment",
            "test_rules.py:1": "test", "p.go:2": "text",
        })
        root = os.path.join(self.tmp.name, "nested\n")
        os.mkdir(root)
        repo = Repo(root)
        repo.write("README.md", "Base.\n")
        repo.commit("base")
        repo.write("rule.md", "A rule.\n")
        rows = repo.rows("--base", "main", "--scope", "changed")
        self.assertEqual([row[3] for row in rows], ["rule.md:1"])

    def test_round_nineteen_cases(self):
        self.repo.write("web/quota.test.ts", 'const ratio = 8 / `requests/second\npending`.length;\ntest("rejects", () => {});\n')
        self.repo.write("web/greek.ts", "const \u037a = 1;\n// Never overwrite.\n")
        rows = self.located(self.repo.rows("--base", "main"))
        self.assertEqual({loc: row[1] for loc, row in rows.items()}, {
            "web/quota.test.ts:3": "test", "web/greek.ts:2": "comment",
        })

    def test_round_twenty_cases(self):
        self.repo.write("web/quota.test.ts", 'const ratio = (8) / `requests/second\npending`.length;\n'
                        'test("rejects invalid input", () => {});\n// Never overwrite.\n'
                        'const n = +function(){} / `per/second\nnever`.length;\n// Still read.\n')
        with open(os.path.join(self.repo.root, "message.ts"), "wb") as handle:
            handle.write(b"// Comment.\rconst message = `first line\nNever overwrite.\n`;\n")
        with open(os.path.join(self.repo.root, "message.py"), "wb") as handle:
            handle.write(b'# Comment.\rmessage = """first line\nNever overwrite.\n"""\n')
        rows = self.located(self.repo.rows("--base", "main"))
        self.assertEqual({loc: row[1] for loc, row in rows.items()}, {
            "web/quota.test.ts:3": "test", "web/quota.test.ts:4": "comment", "web/quota.test.ts:7": "comment",
            "message.ts:1": "comment", "message.ts:2": "text", "message.ts:3": "text",
            "message.py:1": "comment", "message.py:2": "text", "message.py:3": "text",
        })

    def test_round_twenty_one_cases(self):
        self.repo.write("web/quota.test.ts", 'const ratio = 8 as import("./units").Count<number> / `requests/second\npending`.length;\n'
                        'test("rejects", () => {});\n// Never overwrite.\n')
        with open(os.path.join(self.repo.root, "test_rules.py"), "wb") as handle:
            handle.write(b"import unittest\rclass TestRules(unittest.TestCase):\r def test_never_overwrites(self):\r  self.assertTrue(True)\r")
        rows = self.located(self.repo.rows("--base", "main"))
        self.assertEqual({loc: row[1] for loc, row in rows.items()}, {
            "web/quota.test.ts:3": "test", "web/quota.test.ts:4": "comment", "test_rules.py:1": "test",
        })

    def test_round_twenty_two_cases(self):
        self.repo.write("web/message.ts", "type Box<T> = T;\nconst ratio = 8 as Box<number> / `requests/second\npending`.length;\n"
                        "const help = `Usage\nNever overwrite existing files.\n`;\n")
        rows = self.located(self.repo.rows("--base", "main"))
        for line in (2, 3, 4, 5, 6):
            self.assertIn(f"web/message.ts:{line}", rows)

    def test_a_file_a_sparse_checkout_leaves_out(self):
        self.repo.write("docs/contract.md", "Never overwrite an existing recording.\n")
        self.repo.commit("contract")
        self.repo.git("sparse-checkout", "set", "--no-cone", "/README.md")
        self.assertFalse(os.path.exists(os.path.join(self.repo.root, "docs/contract.md")))
        rows = self.located(self.repo.rows("--base", "main"))
        self.assertEqual(rows["docs/contract.md:1"][:2], ("added", "doc"))

    def test_round_twenty_three_cases(self):
        self.repo.write("web/platform.test.ts", 'test.skipIf("win32" === process.platform)(\n"never overwrites user files", () => {});\n')
        self.repo.write("web/escaped.test.ts", 't\\u0065st("never overwrites files", () => {});\nconst \\u{61}b = 4 / 2; // Only half.\n')
        rows = self.located(self.repo.rows("--base", "main"))
        self.assertEqual({loc: row[1] for loc, row in rows.items()}, {
            "web/platform.test.ts:1": "test", "web/platform.test.ts:2": "test",
            "web/escaped.test.ts:1": "test", "web/escaped.test.ts:2": "comment",
        })

    def test_round_twenty_four_cases(self):
        self.repo.write("web/quota.test.ts", 'const n\u200c = 8;\nconst ratio = n\u200c / `requests/second\npending`.length;\n'
                        '// Never overwrite.\ntest("rejects", () => {});\n')
        with open(os.path.join(self.repo.root, "test_rules.py"), "wb") as handle:
            handle.write(b"# coding: latin-1\ndef test_caf\xe9():\n    pass\n")
        self.repo.write("pkg/a_test.go", 'package p\nimport "testing"\nfunc/* contract */TestNeverOverwrites(t *testing.T) {}\n')
        rows = self.located(self.repo.rows("--base", "main"))
        self.assertRowsInclude(rows, {
            "web/quota.test.ts:4": "comment", "web/quota.test.ts:5": "test",
            "test_rules.py:1": "comment", "test_rules.py:2": "test", "pkg/a_test.go:3": "test",
        })
        self.repo.commit("head")
        rows = self.located(self.repo.rows("--base", "main", "--head", "HEAD"))
        self.assertEqual(rows["test_rules.py:2"][1], "test", "a committed Python file keeps its declared encoding")

    def test_round_twenty_five_cases(self):
        self.repo.write("web/quota.ts", "const ratio = 8\n / `requests/second\n" + "Only accepted requests count.\n" * 36 + "`.length;\n")
        self.repo.write("doc.md", "We must\tnot overwrite existing files.\nUse A | B.\n")
        rows = self.located(self.repo.rows("--base", "main"))
        for line in range(2, 40):
            self.assertIn(f"web/quota.ts:{line}", rows)
        tsv = self.repo.rows("--base", "main", "--scope", "changed", "--hint", r"\bmust\s+not\b")
        self.assertTrue(all(len(row) == 5 for row in tsv))
        md = self.repo.run("--base", "main", "--scope", "changed", "--hint", r"\|", "--format", "md").stdout.splitlines()
        self.assertIn("| added | doc | \\| | doc.md:2 | Use A \\| B. |", md)

    def test_a_test_inside_an_outer_call_s_first_argument(self):
        self.repo.write("web/suite.test.ts", 'describe(it("never overwrites", () => {}).title, () => {\n  test("keeps", () => {});\n});\n')
        rows = self.located(self.repo.rows("--base", "main"))
        self.assertEqual({loc: row[1] for loc, row in rows.items()}, {
            "web/suite.test.ts:1": "test", "web/suite.test.ts:2": "test",
        })

    def test_a_regex_multiplied_or_divided(self):
        self.repo.write("web/sample.test.ts", 'const result = /[`]/*2;\ntest("never overwrites", () => {});\n// Never overwrite user files.\n'
                        'const other = /[`]//2;\ntest("keeps", () => {});\n')
        rows = self.located(self.repo.rows("--base", "main"))
        self.assertRowsInclude(rows, {
            "web/sample.test.ts:2": "test", "web/sample.test.ts:3": "comment", "web/sample.test.ts:5": "test",
        })

    def test_round_twenty_eight_cases(self):
        self.repo.git("checkout", "-q", "main")
        self.repo.write("test_rules.py", "def \\\n    test_accepts_invalid():\n    pass\n")
        self.repo.commit("rules")
        self.repo.git("checkout", "-q", "change")
        self.repo.git("reset", "-q", "--hard", "main")
        self.repo.write("test_rules.py", "def \\\n    test_rejects_invalid():\n    pass\n")
        self.repo.write("web/unicode.ts", "const \u2118 = 8; const ratio = \u2118 / `requests/second\nNever count refused requests.\n`.length;\n")
        self.repo.write("web/private.ts", "class C { #\\u0069n = 8; f() { return this.#\\u0069n / `requests/second\n"
                        "Never count refused requests.\n`.length; } }\n")
        self.repo.write("web/table.test.ts", 'test.each`value\n${test("never overwrites", () => {})}\n`("rejects", () => {});\n')
        rows = self.repo.rows("--base", "main", "--scope", "changed")
        self.assertIn(("added", "test", "-", "test_rules.py:2", "test_rejects_invalid():"), rows)
        self.assertIn(("deleted", "test", "-", "test_rules.py:2", "test_accepts_invalid():"), rows)
        located = self.located(rows)
        self.assertRowsInclude(located, {
            "web/unicode.ts:1": "text", "web/unicode.ts:2": "text", "web/unicode.ts:3": "text",
            "web/private.ts:1": "text", "web/private.ts:2": "text", "web/private.ts:3": "text",
            "web/table.test.ts:1": "test", "web/table.test.ts:2": "test", "web/table.test.ts:3": "test",
        })

    def test_a_slash_another_reversal_reveals(self):
        self.repo.write("web/quota.ts", 'const q = 8 as import("./units").T<0> / `/`.length / 8 as import("./units").T<0> / `c/d\n'
                        "Never lose this.\n`.length;\n")
        rows = self.located(self.repo.rows("--base", "main"))
        self.assertRowsInclude(rows, {"web/quota.ts:1": "text", "web/quota.ts:2": "text", "web/quota.ts:3": "text"})

    def test_a_marker_in_a_later_comment_on_the_line(self):
        self.repo.write("generated.md", "<!-- Licence. --> <!-- Code generated by docsgen; DO NOT EDIT. -->\n\nNever overwrite the input.\n")
        self.repo.write("gen.go", "/* Licence. */ // Code generated by gen; DO NOT EDIT.\npackage p\n// A rule.\n")
        self.repo.write("hand.md", "<!-- Licence. --> Text.\n<!-- Code generated by docsgen; DO NOT EDIT. -->\n")
        result = self.repo.run("--base", "main", "--scope", "changed")
        rows = self.located([tuple(line.split("\t")) for line in result.stdout.splitlines()])
        self.assertEqual(sorted(rows), ["hand.md:1", "hand.md:2"])
        self.assertIn("generated files have no rows: gen.go, generated.md", result.stderr)

    def test_round_thirty_two_cases(self):
        self.repo.git("checkout", "-q", "main")
        with open(os.path.join(self.repo.root, "test_rules.py"), "wb") as handle:
            handle.write(b"VALUE = 1 # note\rdef \\\n    test_accepts_invalid():\n    pass\n")
        self.repo.commit("rules")
        self.repo.git("checkout", "-q", "change")
        self.repo.git("reset", "-q", "--hard", "main")
        self.repo.write("web/quota.test.ts", 'const a = 8 as import("./u").T<0> / `a/b\n`.length; const b = 8 as import("./u").T<0> / `c/d\n'
                        '`.length; test("rejects", () => {});\n')
        with open(os.path.join(self.repo.root, "test_rules.py"), "wb") as handle:
            handle.write(b"VALUE = 1 # note\rdef \\\n    test_rejects_invalid():\n    pass\n")
        rows = self.repo.rows("--base", "main", "--scope", "changed")
        located = {(row[0], row[3]): row[1] for row in rows}
        self.assertEqual(located.get(("added", "web/quota.test.ts:3")), "test")
        self.assertEqual(located.get(("added", "test_rules.py:2")), "test")
        self.assertEqual(located.get(("deleted", "test_rules.py:2")), "test")

    def test_hints_order_files_and_keep_lines_in_order(self):
        self.repo.write("a.md", "Plain.\nAlso plain.\n")
        self.repo.write("b.md", "Second line.\nIt must hold.\n")
        rows = [row for row in self.repo.rows("--base", "main", "--scope", "changed")]
        self.assertEqual([row[3] for row in rows], ["b.md:1", "b.md:2", "a.md:1", "a.md:2"])

    def test_the_repository_is_not_changed(self):
        self.repo.write("README.md", "# Project\n\nChanged.\n")
        before = self.repo.git("status", "--porcelain", "--ignored")
        self.repo.run("--base", "main")
        self.assertEqual(self.repo.git("status", "--porcelain", "--ignored"), before)


def repository_root():
    return subprocess.run(
        ["git", "-C", HERE, "rev-parse", "--show-toplevel"], capture_output=True, text=True, check=True,
    ).stdout.rstrip("\n")


def tracked(root, pattern):
    out = subprocess.run(
        ["git", "-C", root, "ls-files", "-z", pattern], capture_output=True, text=True, check=True,
    ).stdout
    return [os.path.join(root, p) for p in out.split("\0") if p and "/testdata/" not in f"/{p}"]


def sample(paths):
    paths = sorted(paths)
    return paths if EXHAUSTIVE else paths[::SAMPLE_EVERY]


def read_source(path):
    with open(path, encoding="utf-8", errors="replace", newline="") as handle:
        return handle.read()


# Python 3.12 tokenizes an f-string into its parts; earlier versions give
# one STRING token, which says nothing of its fields.
FSTRING_PARTS = hasattr(tokenize, "FSTRING_MIDDLE")


def is_fstring(tok):
    return tok.type == tokenize.STRING and "f" in tok.string[:2].lower().replace('"', "").replace("'", "")


def tokenize_sources(text):
    """The line sources Python's own tokenizer implies, by the rules the
    inventory documents: comments, literal text that spans lines or holds
    two words, and test declarations at the start of a statement. Returns
    them with the lines it cannot judge: before Python 3.12, the lines an
    f-string spans."""
    kinds = {}
    unjudged = set()

    def add(line, kind):
        kinds.setdefault(line, set()).add(kind)

    def literal(start, end, body):
        if end > start:
            for line in range(start, end + 1):
                add(line, "text")
        elif inventory.WORDS.search(body):
            add(start, "text")

    tokens = list(tokenize.generate_tokens(io.StringIO(text).readline))
    piece = None  # consecutive FSTRING_MIDDLE tokens: [start line, end line, text]
    for i, tok in enumerate(tokens):
        if FSTRING_PARTS and tok.type == tokenize.FSTRING_MIDDLE:
            if piece is None:
                piece = [tok.start[0], tok.end[0], tok.string]
            else:
                piece[1], piece[2] = tok.end[0], piece[2] + tok.string
            continue
        if piece is not None:
            literal(*piece)
            piece = None
        if FSTRING_PARTS and tok.type in (tokenize.FSTRING_START, tokenize.FSTRING_END):
            # A literal part that runs past a line is one FSTRING_MIDDLE
            # run across those lines; the delimiters add nothing.
            continue
        if tok.type == tokenize.COMMENT:
            add(tok.start[0], "comment")
        elif tok.type == tokenize.STRING:
            if is_fstring(tok):
                unjudged.update(range(tok.start[0], tok.end[0] + 1))
                continue
            body = tok.string.lstrip("rRbBuU")
            quote = 3 if body[:3] in ('"""', "'''") else 1
            literal(tok.start[0], tok.end[0], body[quote:-quote])
        elif tok.type == tokenize.NAME and tok.string in ("def", "class"):
            j = i - 1
            while j >= 0 and tokens[j].type in (tokenize.INDENT, tokenize.DEDENT):
                j -= 1
            starts = j < 0 or tokens[j].type in (tokenize.NEWLINE, tokenize.NL) or (
                tokens[j].string == "async" and tokens[j].start[0] == tok.start[0]
            )
            name = tokens[i + 1] if i + 1 < len(tokens) else None
            if not (starts and name and name.type == tokenize.NAME):
                continue
            if tok.string == "def" and name.string.startswith("test"):
                # From def through the name's line: def \\ then the name.
                for line in range(tok.start[0], name.start[0]):
                    add(line, "test")
                j = i + 2
                if tokens[j].string == "[":
                    # PEP 695 type parameters, def test_x[T](, up to their ].
                    level = 0
                    while j < len(tokens):
                        level += {"[": 1, "]": -1}.get(tokens[j].string, 0)
                        j += 1
                        if level == 0:
                            break
                if j < len(tokens) and tokens[j].string == "(":
                    add(name.start[0], "test")
                else:
                    for line in range(tok.start[0], name.start[0]):
                        kinds[line].discard("test")
            if tok.string == "class" and name.string.startswith("Test"):
                for line in range(tok.start[0], name.start[0] + 1):
                    add(line, "test")
    lines = text.split("\n")
    found = {
        line: inventory.pick(kind)
        for line, kind in kinds.items()
        if line - 1 < len(lines) and lines[line - 1].strip()
    }
    return found, unjudged


class LexerOracleTest(unittest.TestCase):
    """The inventory's lexers agree, line for line, with each language's
    own scanner on real files: the repository's, and the standard
    library's."""

    def test_go_lexer_matches_go_scanner(self):
        root = repository_root()
        goroot = subprocess.run(["go", "env", "GOROOT"], capture_output=True, text=True, check=True).stdout.strip()
        stdlib = []
        for directory, dirs, names in os.walk(os.path.join(goroot, "src")):
            dirs[:] = [d for d in dirs if d != "testdata"]
            stdlib += [os.path.join(directory, n) for n in names if n.endswith(".go")]
        paths = tracked(root, "*.go") + sample(stdlib)
        oracle = subprocess.run(
            ["go", "run", "./tools/sweeporacle"], cwd=root, input="\n".join(paths) + "\n",
            capture_output=True, text=True, check=True,
        ).stdout
        want = {}
        for row in oracle.splitlines():
            location, kind = row.rsplit("\t", 1)
            path, line = location.rsplit(":", 1)
            want.setdefault(path, {})[int(line)] = kind
        lines = 0
        for path in paths:
            got = inventory.go_sources(inventory.split_lines(read_source(path)))
            self.assertEqual(got, want.get(path, {}), path)
            lines += len(got)
        self.assertGreater(lines, 10000, "the oracle compared too little to mean anything")

    def test_go_oracle_reports_physical_lines(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = os.path.join(tmp, "line.go")
            with open(path, "w", encoding="utf-8") as handle:
                handle.write("package p\n//line template.tmpl:100\n// Never overwrite.\n")
            out = subprocess.run(
                ["go", "run", "./tools/sweeporacle"], cwd=repository_root(), input=path + "\n",
                capture_output=True, text=True, check=True,
            ).stdout
        self.assertEqual(out.splitlines(), [f"{path}:2\tcomment", f"{path}:3\tcomment"])

    def test_typescript_lexer_matches_the_compiler(self):
        compiler = os.environ.get("ARCLINT_TYPESCRIPT")
        if not compiler:
            self.skipTest("set ARCLINT_TYPESCRIPT to a typescript.js; see tools/sweeporacle/README.md")
        root = repository_root()
        library = os.path.dirname(compiler)
        paths = tracked(root, "*.ts") + sorted(
            os.path.join(library, n) for n in os.listdir(library) if n.endswith((".d.ts", ".js"))
        )
        oracle = subprocess.run(
            ["node", "--max-old-space-size=8000", os.path.join(root, "tools", "sweeporacle", "typescript.cjs")],
            input="\n".join(paths) + "\n", capture_output=True, text=True, check=True,
            env={**os.environ, "TYPESCRIPT": compiler},
        ).stdout
        want = {}
        for row in oracle.splitlines():
            location, kind = row.rsplit("\t", 1)
            path, line = location.rsplit(":", 1)
            want.setdefault(path, {})[int(line)] = kind
        lines = 0
        for path in paths:
            got = inventory.ts_sources(inventory.split_lines(read_source(path)))
            self.assertEqual(got, want.get(path, {}), path)
            lines += len(got)
        self.assertGreater(lines, 10000, "the oracle compared too little to mean anything")

    def test_python_lexer_matches_tokenize(self):
        root = repository_root()
        stdlib_dir = sysconfig.get_paths()["stdlib"]
        stdlib = []
        for directory, dirs, names in os.walk(stdlib_dir):
            dirs[:] = [d for d in dirs if d not in ("site-packages", "lib2to3")]
            stdlib += [os.path.join(directory, n) for n in names if n.endswith(".py")]
        lines = 0
        for path in tracked(root, "*.py") + sample(stdlib):
            text = read_source(path)
            want, unjudged = tokenize_sources(text)
            got = inventory.python_sources(inventory.split_lines(text))
            got = {line: kind for line, kind in got.items() if line not in unjudged}
            want = {line: kind for line, kind in want.items() if line not in unjudged}
            self.assertEqual(got, want, path)
            lines += len(got)
        self.assertGreater(lines, 1000, "the oracle compared too little to mean anything")


if __name__ == "__main__":
    unittest.main()
