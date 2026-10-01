import importlib.util
import json
from pathlib import Path
import sys
import subprocess
from unittest import mock
import tempfile
import unittest

sys.dont_write_bytecode = True
spec = importlib.util.spec_from_file_location("guard", Path(__file__).with_name("guard.py"))
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)


class GuardTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        (self.root/".arclint").mkdir()
        (self.root/".codex/hooks/arclint-domain-guard").mkdir(parents=True)
        (self.root/".codex/hooks/arclint-domain-guard/guard.py").write_text("fixture")
        (self.root/".codex/hooks.json").write_text("{}")
        (self.root/".arclint/domain-guard.json").write_text(json.dumps({
            "version":1,"domainFiles":["domain.yaml"],"sourcePatterns":["src/*.go"]}))
        (self.root/"domain.yaml").write_text("contexts: []\n")
        (self.root/"src").mkdir()
        (self.root/"src/domain.go").write_text("package domain\n")
        self.state = {}
        self.run_event("SessionStart")

    def run_event(self, name, reviewer=None, **fields):
        return guard.handle(self.root, dict(hook_event_name=name, **fields), self.state,
                            reviewer or (lambda evidence, previous: {"relevant":True,"findings":[]}))

    def test_source_contents_are_reviewed_and_fingerprinted(self):
        seen = []
        def review(evidence, previous):
            seen.append(evidence)
            return {"relevant":True,"findings":[]}
        self.run_event("UserPromptSubmit", prompt="Change domain", reviewer=review)
        self.run_event("Stop",last_assistant_message="Done",reviewer=review)
        first = self.state["verdict"]["fingerprint"]
        (self.root/"src/domain.go").write_text("package changed\n")
        self.run_event("Stop",last_assistant_message="Done",reviewer=review)
        self.assertNotEqual(first,self.state["verdict"]["fingerprint"])
        self.assertEqual(seen[-1]["src/domain.go"],"package changed\n")

    def test_proposed_new_source_category_blocks_before_edit(self):
        self.run_event("UserPromptSubmit",prompt="Use the existing Rule")
        def review(evidence, previous):
            return {"relevant":True,"findings":[{"file":"proposal","quote":"NewCategory",
                    "reason":"The request asks for a Rule.","repair":"Reuse Rule."}]}
        result=self.run_event("PreToolUse",tool_name="apply_patch",tool_input={"command":"src/domain.go NewCategory"},reviewer=review)
        self.assertEqual(result["hookSpecificOutput"]["permissionDecision"],"deny")
        self.assertEqual((self.root/"src/domain.go").read_text(),"package domain\n")

    def test_response_only_findings_and_rebuttal(self):
        self.run_event("UserPromptSubmit",prompt="Explain the Rule")
        def review(evidence, previous):
            return {"relevant":True,"findings":[{"file":"response","quote":"NewCategory",
                    "reason":"Unrequested category.","repair":"Explain the existing Rule."}]}
        result=self.run_event("Stop",last_assistant_message="NewCategory",reviewer=review)
        self.assertEqual(result["decision"],"block")
        self.run_event("UserPromptSubmit",prompt=result["reason"])
        self.assertEqual(self.state["requests"],["Explain the Rule"])
        self.run_event("Stop",last_assistant_message="Here is the existing Rule and supporting evidence.")
        self.assertEqual(self.state["verdict"]["status"],"passed")

    def test_comment_findings_reach_builder_and_repair_passes(self):
        self.run_event("UserPromptSubmit",prompt="Update domain")
        (self.root/"domain.yaml").write_text("# commentary\ncontexts: []\n")
        result=self.run_event("PostToolUse")
        self.assertIn("commentary",result["hookSpecificOutput"]["additionalContext"])
        self.assertEqual(self.run_event("Stop",last_assistant_message="Done")["decision"],"block")
        (self.root/"domain.yaml").write_text("contexts: []\n")
        self.run_event("PostToolUse")
        self.assertEqual(self.run_event("Stop",last_assistant_message="Removed comment"),{})

    def test_rules_and_baseline_changes_invalidate(self):
        for name in ["rules.arclint.yaml",".arclint/baseline.v2.json"]:
            path=self.root/name
            path.write_text("changed")
            with self.assertRaisesRegex(ValueError,"rules or baseline changed"):
                self.run_event("Stop",last_assistant_message="Done")
            path.unlink()

    def test_configuration_and_code_changes_invalidate(self):
        path=self.root/".codex/hooks/arclint-domain-guard/guard.py"
        path.write_text("tampered")
        with self.assertRaisesRegex(ValueError,"hook code"):
            self.run_event("Stop",last_assistant_message="Done")

    def test_missing_source_pattern_cannot_pass(self):
        config=json.loads((self.root/".arclint/domain-guard.json").read_text())
        config["sourcePatterns"]=["src/missing*.go"]
        with self.assertRaisesRegex(ValueError,"no files"):
            guard.scoped_files(self.root,config)

    def test_source_cannot_escape_project(self):
        config={"domainFiles":["domain.yaml"],"sourcePatterns":["../*.go"]}
        with self.assertRaisesRegex(ValueError,"literal directory prefix"):
            guard.scoped_files(self.root,config)

    def test_no_approval_when_source_changes_during_review(self):
        def review(evidence, previous):
            (self.root/"src/domain.go").write_text("package raced\n")
            return {"relevant":True,"findings":[]}
        with self.assertRaisesRegex(ValueError,"stale"):
            self.run_event("Stop",last_assistant_message="Done",reviewer=review)

    def test_stalled_review_preserves_operator_escape(self):
        for _ in range(2):
            self.assertEqual(guard.deny("Stop","Unresolved finding",self.state)["decision"],"block")
        result=guard.deny("Stop","Unresolved finding",self.state)
        self.assertFalse(result["continue"])
        self.assertIn("without approval",result["stopReason"])

    def test_unverifiable_verdict_never_passes(self):
        def review(evidence, previous):
            return {"relevant":True,"findings":[{"file":"response","quote":"not present","reason":"x","repair":"y"}]}
        with self.assertRaisesRegex(ValueError,"verifiable"):
            self.run_event("Stop",last_assistant_message="Done",reviewer=review)


    def test_startup_failure_blocks_edit_and_pauses_completion(self):
        script = Path(guard.__file__)
        for event in ("PreToolUse", "Stop"):
            result = subprocess.run([sys.executable, str(script), "--root", str(self.root/"missing")],
                                    input=json.dumps({"hook_event_name":event}), text=True,
                                    capture_output=True, check=False)
            self.assertEqual(result.returncode, 0)
            output = json.loads(result.stdout)
            if event == "PreToolUse":
                self.assertEqual(output["hookSpecificOutput"]["permissionDecision"], "deny")
            else:
                self.assertFalse(output["continue"])
                self.assertIn("no approval", output["stopReason"])

    def test_invalid_input_returns_blocking_exit_code(self):
        result = subprocess.run([sys.executable, guard.__file__, "--root", str(self.root)],
                                input="{broken", text=True, capture_output=True, check=False)
        self.assertEqual(result.returncode, 2)
        self.assertIn("no approval", result.stderr)

    def test_reviewer_is_ephemeral_read_only_and_nonrecursive(self):
        def run(command, **kwargs):
            self.assertIn("--ephemeral", command)
            self.assertIn("--ignore-user-config", command)
            self.assertEqual(command[command.index("--sandbox")+1], "read-only")
            disabled = [command[i+1] for i, value in enumerate(command) if value == "--disable"]
            self.assertTrue({"hooks", "shell_tool", "multi_agent", "code_mode_host"}.issubset(disabled))
            self.assertEqual(kwargs["timeout"], 70)
            self.assertNotEqual(Path(command[command.index("--cd")+1]), self.root)
            Path(command[command.index("--output-last-message")+1]).write_text(
                '{"relevant":true,"findings":[]}')
            return subprocess.CompletedProcess(command, 0)
        with mock.patch.object(guard.subprocess, "run", side_effect=run):
            self.assertEqual(guard.semantic({"request":"Review"}, [], None),
                             {"relevant":True,"findings":[]})

    def test_go_lexer_ignores_literals_and_keeps_line_numbers(self):
        source = 'package domain\nvar url = "https://host/*path*/"\nvar raw = ' + chr(96) + '// literal\n/* literal */' + chr(96) + '\nvar rune = \'/\'\n// explanation\nvar n = 1 /* inline */\n'
        found = guard.go_comments(source)
        self.assertEqual([x["quote"] for x in found], ["// explanation", "/* inline */"])
        self.assertEqual([x["line"] for x in found], [6, 7])
        escaped = 'package domain\nvar s = "quote: \\" // still string"\n'
        self.assertEqual(guard.go_comments(escaped), [])

    def test_go_policy_preserves_directives_and_legal_headers(self):
        source = ('// Copyright 2026 Example\n// Licensed under the Apache License.\n\n'
                  '// SPDX-License-Identifier: MIT\n\n//go:build linux\n// +build linux\n\n'
                  '// Code generated by example. DO NOT EDIT.\n\npackage domain\n'
                  '//go:embed template.txt\nvar template string\n//line domain.go:1\n'
                  '/* #cgo CFLAGS: -DVALUE=1\n#include <stdint.h> */\nimport "C"\n')
        found = guard.go_comments(source)
        self.assertTrue(found)
        self.assertTrue(all(x["preservedBy"] for x in found))
        self.assertEqual(guard.go_comments('package domain\n// Copyright is a business term\n')[0]["preservedBy"], None)

    def test_domain_source_comments_block_and_repair_without_banning_context(self):
        config_path = self.root/".arclint/domain-guard.json"
        config = json.loads(config_path.read_text())
        config["domainSourcePatterns"] = ["src/domain.go"]
        config_path.write_text(json.dumps(config))
        (self.root/"src/context.go").write_text("package transport\n// HTTP adapter explanation\n")
        self.run_event("SessionStart")
        self.run_event("UserPromptSubmit", prompt="Update the domain")
        (self.root/"src/domain.go").write_text("package domain\n// unwanted explanation\n")
        result = self.run_event("PostToolUse")
        self.assertIn("domain-source", result["hookSpecificOutput"]["additionalContext"])
        self.assertEqual(self.run_event("Stop",last_assistant_message="Done")["decision"], "block")
        (self.root/"src/domain.go").write_text('package domain\nvar endpoint = "https://example.test"\n')
        self.run_event("PostToolUse")
        self.assertEqual(self.run_event("Stop",last_assistant_message="Repaired"), {})
        self.assertIn("// HTTP", (self.root/"src/context.go").read_text())

    def test_domain_subject_scope_is_additive_fingerprinted_and_language_checked(self):
        config = {"domainFiles":["domain.yaml"], "domainSourcePatterns":["src/domain.go"]}
        files = guard.scoped_files(self.root, config)
        self.assertIn("src/domain.go", files)
        before = guard.digest(files)
        (self.root/"src/domain.go").write_text("package changed\n")
        self.assertNotEqual(before, guard.digest(guard.scoped_files(self.root, config)))
        (self.root/"src/other.ts").write_text("const url = 'https://example.test';")
        config["domainSourcePatterns"] = ["src/other.ts"]
        with self.assertRaisesRegex(ValueError, "support .go only"):
            guard.scoped_files(self.root, config)

    def test_directives_survive_adjacent_explanation_and_line_cgo(self):
        source = ('package domain\n//go:embed file.txt\n// explanation\nvar text string\n'
                  '/*line generated.go:2*/\n// #include <stdio.h>\nimport "C"\n')
        found = guard.go_comments(source)
        self.assertEqual([x["quote"] for x in found if not x["preservedBy"]], ["// explanation"])
        self.assertEqual(guard.comments("# yaml-language-server: $schema=domain.json\n# explanation\n"), [(2, "# explanation")])

    def test_unterminated_go_literal_or_comment_cannot_pass(self):
        for source in ['package domain\n/* missing', 'package domain\nvar s = "missing']:
            with self.assertRaisesRegex(ValueError, "Unterminated Go"):
                guard.go_comments(source)



if __name__=="__main__":
    unittest.main()
