"""Tests for inventory.py, each run against a real git repository.

Run: python3 -I -B -m unittest discover -s .agents/skills/domain-sweep/scripts
"""

import os
import subprocess
import sys
import tempfile
import textwrap
import unittest

SCRIPT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "inventory.py")


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


if __name__ == "__main__":
    unittest.main()
