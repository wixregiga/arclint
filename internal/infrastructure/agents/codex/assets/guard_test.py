import importlib.util
import json
import os
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




class GuardProtocolTests(unittest.TestCase):
    """Run the installed script over stdin/stdout; the model executable is a unit fixture."""
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)/"project"
        self.root.mkdir()
        (self.root/".arclint").mkdir()
        (self.root/".codex/hooks/arclint-domain-guard").mkdir(parents=True)
        self.script = self.root/".codex/hooks/arclint-domain-guard/guard.py"
        self.script.write_bytes(Path(guard.__file__).read_bytes())
        (self.root/".codex/hooks.json").write_text("{}")
        (self.root/"domain.yaml").write_text("version: 1\ncontexts: {}\n")
        (self.root/"src").mkdir()
        (self.root/"src/domain.go").write_text("package domain\n")
        self.config = {"version":1,"domainFiles":["domain.yaml"],"sourcePatterns":["src/domain.go"]}
        self.configure()
        self.log = Path(self.tmp.name)/"model-calls.jsonl"
        binary = Path(self.tmp.name)/"codex"
        binary.write_text("#!" + sys.executable + "\n" + r"""
import json,os,sys
from pathlib import Path
prompt = sys.stdin.read()
evidence = json.loads(prompt.split('\nEvidence:\n', 1)[1].split('\nPrevious findings:\n', 1)[0])
with open(os.environ['ARCLINT_UNIT_REVIEW_LOG'], 'a') as stream:
    stream.write(json.dumps(evidence) + '\n')
findings = []
if 'FORBIDDEN_NEW_MEANING' in evidence.get('proposal', ''):
    findings = [{'file':'proposal','quote':'FORBIDDEN_NEW_MEANING','reason':'Unit fixture rejects this proposal.','repair':'Restore only the agreed content.'}]
Path(sys.argv[sys.argv.index('--output-last-message')+1]).write_text(json.dumps({'relevant':True,'findings':findings}))
""")
        binary.chmod(0o700)
        self.env = dict(os.environ, PATH=self.tmp.name+os.pathsep+os.environ["PATH"], ARCLINT_UNIT_REVIEW_LOG=str(self.log))

    def configure(self):
        (self.root/".arclint/domain-guard.json").write_text(json.dumps(self.config))

    def event(self, name, **fields):
        event = dict(hook_event_name=name, session_id="protocol-fixture", **fields)
        result = subprocess.run([sys.executable, "-B", str(self.script), "--root", str(self.root)],
                                input=json.dumps(event), text=True, capture_output=True, env=self.env, check=False)
        self.assertEqual(result.returncode, 0, result.stderr)
        return json.loads(result.stdout)

    def state(self):
        return json.loads(next((self.root/".arclint/cache/codex-domain-guard").glob("*.json")).read_text())

    def allowed(self, output):
        self.assertNotEqual(output.get("hookSpecificOutput", {}).get("permissionDecision"), "deny", output)
        self.assertNotEqual(output.get("continue"), False, output)

    def restore(self, path, content):
        patch = "*** Begin Patch\n*** Add File: " + path + "\n" + "".join("+"+line+"\n" for line in content.splitlines()) + "*** End Patch"
        return self.event("PreToolUse", tool_name="apply_patch", tool_input={"input":patch})

    def test_missing_recording_restores_through_pre_post_and_fresh_stop(self):
        self.missing_lifecycle("domain.yaml", "version: 1\ncontexts: {}\n")

    def test_missing_literal_source_restores_through_pre_post_and_fresh_stop(self):
        self.missing_lifecycle("src/domain.go", "package domain\n")

    def test_custom_recording_under_arclint_restores_without_metadata_exemption(self):
        self.config["domainFiles"] = [".arclint/domain.yaml"]
        self.configure()
        (self.root/".arclint/domain.yaml").write_text("version: 1\ncontexts: {}\n")
        self.missing_lifecycle(".arclint/domain.yaml", "version: 1\ncontexts: {}\n")

    def test_selecting_missing_metadata_does_not_make_it_restorable(self):
        for name in ("rules.arclint.yaml", ".arclint/baseline.v2.json", ".arclint/agent-assets.json", ".arclint/reviewer.json", ".arclint/cache/fake.json", ".codex/config.toml"):
            with self.subTest(name=name):
                self.config["domainFiles"] = [name]
                self.configure()
                self.event("SessionStart")
                result = self.restore(name, "weakened")
                self.assertEqual(result["hookSpecificOutput"]["permissionDecision"], "deny")
                self.assertIn("protected", result["hookSpecificOutput"]["permissionDecisionReason"])
                self.assertFalse((self.root/name).exists())

    def test_empty_glob_restores_through_pre_post_and_fresh_stop(self):
        self.config["sourcePatterns"] = ["src/**/*.go"]
        self.configure()
        self.missing_lifecycle("src/domain.go", "package domain\n")

    def missing_lifecycle(self, name, content):
        (self.root/name).unlink()
        start = self.event("SessionStart")
        self.assertIn("Missing configured material", start["systemMessage"])
        self.event("UserPromptSubmit", prompt="Restore the missing scoped material with its agreed meaning.")
        self.assertEqual(self.event("Stop", last_assistant_message="Not restored yet")["decision"], "block")
        self.allowed(self.restore(name, content))
        self.assertEqual(self.state()["verdict"]["status"], "permitted")
        self.assertIn("not approval", self.state()["error"])
        self.assertEqual(self.event("Stop", last_assistant_message="Permission is not restoration")["decision"], "block")
        (self.root/name).write_text(content)
        self.event("PostToolUse")
        self.assertEqual(self.state()["verdict"]["event"], "PostToolUse")
        self.assertEqual(self.event("Stop", last_assistant_message="Restored and reviewed"), {})
        evidence = [json.loads(line) for line in self.log.read_text().splitlines()]
        self.assertEqual([item["review_stage"] for item in evidence], ["PreToolUse", "PostToolUse", "Stop"])
        self.assertTrue(json.loads(evidence[0]["missing_material"]))
        self.assertEqual(evidence[-1][name], content)
        self.assertEqual(json.loads(evidence[-1]["missing_material"]), [])

    def test_ordinary_finding_repair_passes_permission_before_mutation(self):
        (self.root/"domain.yaml").write_text("# remove this comment\nversion: 1\n")
        self.event("SessionStart")
        self.event("UserPromptSubmit", prompt="Repair the domain comment.")
        self.assertEqual(self.event("Stop", last_assistant_message="Done")["decision"], "block")
        content = "version: 1\ncontexts: {}\n"
        self.allowed(self.event("PreToolUse", tool_name="Write", tool_input={"file_path":str(self.root/"domain.yaml"), "content":content}))
        self.assertTrue(any(item["open"] for item in self.state()["findings"].values()))
        self.assertEqual(self.state()["verdict"]["status"], "permitted")
        (self.root/"domain.yaml").write_text(content)
        self.event("PostToolUse")
        self.assertEqual(self.event("Stop", last_assistant_message="Repaired comment"), {})
        self.assertFalse(any(item["open"] for item in self.state()["findings"].values()))

    def test_restoration_is_reviewed_and_mixed_or_opaque_edits_are_denied(self):
        (self.root/"src/domain.go").unlink()
        self.event("SessionStart")
        rejected = self.restore("src/domain.go", "FORBIDDEN_NEW_MEANING")
        self.assertEqual(rejected["hookSpecificOutput"]["permissionDecision"], "deny")
        self.assertIn("Unit fixture rejects", rejected["hookSpecificOutput"]["permissionDecisionReason"])
        proposals = [
            ("exec_command", {"cmd":"touch src/domain.go"}),
            ("Write", {"file_path":"unrelated.go", "content":"package unrelated"}),
            ("Write", {"file_path":"domain.yaml", "content":"changed"}),
            ("Write", {"file_path":"rules.arclint.yaml", "content":"weaker"}),
            ("Write", {"file_path":"../outside.go", "content":"outside"}),
            ("apply_patch", {"input":"*** Begin Patch\n*** Add File: src/domain.go\n+package domain\n*** Add File: unrelated.go\n+package unrelated\n*** End Patch"}),
            ("apply_patch", {"input":"*** Begin Patch\n*** Add File: src/domain.go\n+package domain\n*** Update File: domain.yaml\n@@\n-changed\n+also changed\n*** End Patch"}),
        ]
        for tool, inputs in proposals:
            with self.subTest(tool=tool, inputs=inputs):
                output = self.event("PreToolUse", tool_name=tool, tool_input=inputs)
                self.assertEqual(output["hookSpecificOutput"]["permissionDecision"], "deny")
        self.assertFalse((self.root/"src/domain.go").exists())

    def test_inspection_does_not_renew_governance_approval(self):
        self.event("SessionStart")
        self.event("UserPromptSubmit", prompt="Review domain")
        self.assertEqual(self.event("Stop", last_assistant_message="Reviewed"), {})
        (self.root/"rules.arclint.yaml").write_text("changed")
        self.allowed(self.event("PreToolUse", tool_name="Read", tool_input={"file_path":"rules.arclint.yaml"}))
        self.assertEqual(self.state()["verdict"]["status"], "unavailable")
        self.assertEqual(self.event("PreToolUse", tool_name="exec_command", tool_input={"cmd":"make check-ro"})["hookSpecificOutput"]["permissionDecision"], "deny")
        self.assertEqual(self.event("Stop", last_assistant_message="Read is not approval")["decision"], "block")
        (self.root/"rules.arclint.yaml").unlink()
        self.assertEqual(self.event("Stop", last_assistant_message="Restored original governance and reviewed again"), {})

    def another_checkout(self, name="worktree"):
        target = Path(self.tmp.name)/name
        target.mkdir()
        (target/".git").write_text("gitdir: ../project/.git/worktrees/fixture\n")
        (target/".arclint").mkdir()
        (target/".arclint/domain-guard.json").write_text(json.dumps({"version":1,"domainFiles":["domain.yaml"]}))
        (target/"domain.yaml").write_text("version: 1\nproject: other-checkout\n")
        (target/"nested").mkdir()
        return target

    def test_executable_routes_host_cwd_to_its_own_scope_and_state(self):
        self.event("SessionStart")
        original_state = self.state()
        target = self.another_checkout()
        cwd = str(target/"nested")
        self.event("SessionStart", cwd=cwd)
        self.event("UserPromptSubmit", cwd=cwd, prompt="Review this checkout")
        self.assertEqual(self.event("Stop", cwd=cwd, last_assistant_message="Reviewed current checkout"), {})
        evidence = [json.loads(line) for line in self.log.read_text().splitlines()]
        self.assertEqual(evidence[-1]["domain.yaml"], "version: 1\nproject: other-checkout\n")
        self.assertEqual(self.state(), original_state)
        state_path = next((target/".arclint/cache/codex-domain-guard").glob("*.json"))
        self.assertEqual(json.loads(state_path.read_text())["verdict"]["status"], "passed")

    def test_executable_translates_both_wsl_unc_forms_and_rejects_other_distribution(self):
        target = self.another_checkout()
        self.env["WSL_DISTRO_NAME"] = "Fixture-Distro"
        for host in ("wsl$", "wsl.localhost"):
            cwd = "\\\\" + host + "\\Fixture-Distro" + str(target/"nested").replace("/", "\\")
            self.allowed(self.event("SessionStart", cwd=cwd))
            self.event("UserPromptSubmit", cwd=cwd, prompt="Review current worktree")
            self.assertEqual(self.event("Stop", cwd=cwd, last_assistant_message="Reviewed"), {})
        wrong = "\\\\wsl.localhost\\Other-Distro" + str(target).replace("/", "\\")
        rejected = self.event("PreToolUse", cwd=wrong, tool_name="Write", tool_input={"file_path":"domain.yaml","content":"wrong checkout"})
        self.assertEqual(rejected["hookSpecificOutput"]["permissionDecision"], "deny")
        self.assertIn("distribution", rejected["hookSpecificOutput"]["permissionDecisionReason"])

    def test_executable_stops_at_repository_boundary_without_fallback(self):
        self.event("SessionStart")
        previous = self.state()
        nested = self.root/"nested-repo"
        nested.mkdir()
        (nested/".git").write_text("gitdir: somewhere\n")
        result = self.event("PreToolUse", cwd=str(nested), tool_name="Write", tool_input={"file_path":"domain.yaml","content":"wrong checkout"})
        self.assertEqual(result["hookSpecificOutput"]["permissionDecision"], "deny")
        self.assertIn("no domain guard configuration", result["hookSpecificOutput"]["permissionDecisionReason"])
        self.assertEqual(self.state(), previous)
        self.assertFalse((nested/".arclint/cache").exists())
        self.assertFalse(self.log.exists())

    def test_executable_rejects_missing_cwd_instead_of_reviewing_fallback(self):
        self.event("SessionStart")
        previous = self.state()
        result = self.event("Stop", cwd=str(self.root/"absent"), last_assistant_message="Not reviewed")
        self.assertFalse(result["continue"])
        self.assertIn("no approval", result["stopReason"])
        self.assertEqual(self.state(), previous)
        self.assertFalse(self.log.exists())

    def test_executable_fingerprints_actual_inherited_hook_code(self):
        target = self.another_checkout()
        self.event("SessionStart", cwd=str(target))
        self.script.write_text(self.script.read_text()+"\n# changed executing hook\n")
        result = self.event("PreToolUse", cwd=str(target), tool_name="Write", tool_input={"file_path":"domain.yaml","content":"changed"})
        self.assertEqual(result["hookSpecificOutput"]["permissionDecision"], "deny")
        self.assertIn("hook code", result["hookSpecificOutput"]["permissionDecisionReason"])

    def test_executable_fingerprints_inherited_hook_provider_configuration(self):
        target = self.another_checkout()
        self.event("SessionStart", cwd=str(target))
        (self.root/".codex/hooks.json").write_text('{"hooks":{}}')
        result = self.event("PreToolUse", cwd=str(target), tool_name="Write", tool_input={"file_path":"domain.yaml","content":"changed"})
        self.assertEqual(result["hookSpecificOutput"]["permissionDecision"], "deny")
        self.assertIn("hook code", result["hookSpecificOutput"]["permissionDecisionReason"])
        self.assertFalse((target/".codex/hooks.json").exists())

    def test_inspection_before_session_start_leaves_usable_state(self):
        self.allowed(self.event("PreToolUse", tool_name="Read", tool_input={"file_path":"domain.yaml"}))
        self.event("UserPromptSubmit", prompt="Review this domain")
        self.assertEqual(self.event("Stop", last_assistant_message="Reviewed"), {})
        self.assertEqual(self.state()["requests"], ["Review this domain"])

    def test_goal_bookkeeping_is_allowed_but_does_not_accept_governance(self):
        self.event("SessionStart")
        (self.root/"rules.arclint.yaml").write_text("changed")
        self.allowed(self.event("PreToolUse", tool_name="update_goal", tool_input={"status":"blocked"}))
        self.assertEqual(self.state()["verdict"]["status"], "unavailable")
        self.assertEqual(self.event("Stop", last_assistant_message="Still unavailable")["decision"], "block")

    def test_inspection_detects_missing_material_without_approving_it(self):
        self.event("SessionStart")
        self.event("UserPromptSubmit", prompt="Review domain")
        self.assertEqual(self.event("Stop", last_assistant_message="Reviewed"), {})
        (self.root/"src/domain.go").unlink()
        self.allowed(self.event("PreToolUse", tool_name="Read", tool_input={"file_path":"domain.yaml"}))
        self.assertEqual(self.state()["verdict"]["status"], "unavailable")
        self.assertEqual(self.event("Stop", last_assistant_message="Still missing")["decision"], "block")

    def test_in_project_symlink_alias_cannot_restore_protected_governance(self):
        (self.root/"alias").symlink_to(self.root, target_is_directory=True)
        self.config["domainFiles"] = ["alias/rules.yaml"]
        self.configure()
        self.event("SessionStart")
        result = self.restore("alias/rules.yaml", "weakened")
        self.assertEqual(result["hookSpecificOutput"]["permissionDecision"], "deny")
        self.assertIn("symlink", result["hookSpecificOutput"]["permissionDecisionReason"])
        self.assertFalse((self.root/"rules.yaml").exists())

    def test_escaping_symlink_is_not_missing_restorable_material(self):
        (self.root/"src/domain.go").unlink()
        (self.root/"src").rmdir()
        outside = Path(self.tmp.name)/"outside"
        outside.mkdir()
        (self.root/"src").symlink_to(outside, target_is_directory=True)
        self.event("SessionStart")
        result = self.restore("src/domain.go", "package domain\n")
        self.assertEqual(result["hookSpecificOutput"]["permissionDecision"], "deny")
        self.assertIn("escapes", result["hookSpecificOutput"]["permissionDecisionReason"])
        self.assertFalse((outside/"domain.go").exists())

    def test_changed_source_after_post_review_requires_another_review(self):
        self.event("SessionStart")
        self.event("UserPromptSubmit", prompt="Keep the domain clean")
        self.event("PostToolUse")
        (self.root/"domain.yaml").write_text("# late defect\nversion: 1\n")
        self.assertEqual(self.event("Stop", last_assistant_message="Done")["decision"], "block")


if __name__=="__main__":
    unittest.main()
