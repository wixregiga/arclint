#!/usr/bin/env python3
"""Evaluate the shipped named reviewer in a fresh project; never alter existing installs.

Usage: python3 evaluate.py /absolute/path/to/built/arclint /absolute/evidence/output
Requires the built binary to contain the current authored reviewer configuration.
Optional third argument --enable-multi-agent-v2 enables only that host feature
for this invocation; does not change trust, hooks or existing configuration.
No hook disabling, trust bypass, or expectations are passed.
Optional --direct-instructions tests developer instructions through a plain
CLI session and records it as authored-instructions evaluation, not native
named-agent activation. Native mode never injects custom-agent configuration.
"""
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
import time
import tomllib

HERE = Path(__file__).resolve().parent
ROOT = HERE.parent.parent
ASSET = ROOT / "internal/infrastructure/agents/codex/assets/arclint-domain-reviewer.toml"

def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def invoke(command, cwd, output, timeout=30, input_text=None):
    started = time.time()
    try:
        result = subprocess.run(command, cwd=cwd, input=input_text, text=True,
                                capture_output=True, timeout=timeout)
        status = {"exit_code": result.returncode, "timed_out": False}
        output.with_suffix(".stdout").write_text(result.stdout)
        output.with_suffix(".stderr").write_text(result.stderr)
    except subprocess.TimeoutExpired as error:
        status = {"exit_code": None, "timed_out": True}
        output.with_suffix(".stdout").write_bytes(error.stdout or b"")
        output.with_suffix(".stderr").write_bytes(error.stderr or b"")
    status.update(command=command, cwd=str(cwd), elapsed_seconds=round(time.time()-started, 2))
    output.with_suffix(".json").write_text(json.dumps(status, indent=2)+"\n")
    return status

def main():
    binary = Path(sys.argv[1]).resolve(strict=True)
    enable_v2 = "--enable-multi-agent-v2" in sys.argv[3:]
    direct_instructions = "--direct-instructions" in sys.argv[3:]
    evidence = Path(sys.argv[2]).resolve()
    evidence.mkdir(parents=True, exist_ok=False)
    project = Path(tempfile.mkdtemp(prefix="arclint-reviewer-eval-"))
    subprocess.run(["git", "init", "-q", str(project)], check=True)
    shutil.copyfile(ASSET, evidence / "authored.toml")
    cases = HERE / "cases.md"
    inputs = []
    for number, title, text in re.findall(r"## (\d+)\. ([^\n]+)\n(.*?)(?=\n## |\Z)", cases.read_text(), re.S):
        supplied = text.split("\nExpected:", 1)[0].strip()
        if not supplied.startswith("Input:"):
            raise ValueError(f"case {number} has no input")
        inputs.append(f"Case {number}\n{supplied}")
    # The host receives only case inputs. It never receives cases.md or expectations.
    prompt = ("Use the installed custom agent named arclint-domain-reviewer. Spawn it "
              "through the host's native custom-agent mechanism, giving it the following "
              "fourteen independent fictional cases for domain review. Return its actual "
              "response for every case. These are supplied fictional projects: repository "
              "inspection and tools are unnecessary. Do not launch a generic substitute "
              "or read unrelated files. If named discovery is unavailable, report that "
              "failure. Each case is independent; do not carry domain rules from one into "
              "another.\n\n" + "\n\n".join(inputs))
    if direct_instructions:
        prompt = ("Review the following fourteen independent fictional cases. "
                  "Use only the supplied inputs; do not inspect repository files or "
                  "carry a domain rule from one case into another. Return your own "
                  "review for every numbered case.\n\n" + "\n\n".join(inputs))
    (evidence / "inputs.txt").write_text(prompt)
    install = invoke([str(binary), "agents", "reviewer", "install", "--host", "codex"], project, evidence/"install")
    status = invoke([str(binary), "agents", "reviewer", "status", "--format", "json"], project, evidence/"status")
    installed = project / ".codex/agents/arclint-domain-reviewer.toml"
    metadata = {"project": str(project), "authored_sha256": digest(ASSET),
                "binary_sha256": digest(binary), "inputs_sha256": digest(evidence/"inputs.txt"),
                "codex_version": subprocess.check_output(["codex", "--version"], text=True).strip(),
                "expected_outcomes_withheld": True, "evaluation_mode": "authored-instructions" if direct_instructions else "native-name", "installation": install, "status": status}
    if install["exit_code"] != 0 or not installed.exists():
        metadata["evaluation_skipped"] = "installation failed"
        metadata["execution_status"] = "installation-failed"
    elif digest(installed) != digest(ASSET):
        metadata["evaluation_skipped"] = "installed instructions differ from current asset"
        metadata["execution_status"] = "instruction-hash-mismatch"
    else:
        metadata["installed_sha256"] = digest(installed)
        command = ["codex", "exec", "-C", str(project), "--sandbox", "read-only",
                   "--json", "-o", str(evidence/"response.md"), "-"]
        if direct_instructions:
            instructions = tomllib.loads(ASSET.read_text())["developer_instructions"]
            command[2:2] = ["-c", "developer_instructions=" + json.dumps(instructions)]
        if enable_v2:
            command[2:2] = ["--enable", "multi_agent_v2"]
        metadata["multi_agent_v2_override"] = enable_v2
        metadata["invocation"] = invoke(command, project, evidence/"native", timeout=240, input_text=prompt)
        # Keep the host-owned trace when this invocation produced a persisted thread.
        stdout = (evidence/"native.stdout").read_text()
        thread_ids = []
        for line in stdout.splitlines():
            try:
                item = json.loads(line)
            except ValueError:
                continue
            if item.get("type") == "thread.started":
                thread_ids.append(item["thread_id"])
        codex_root = Path(os.environ.get("CODEX_HOME", str(Path.home()/".codex")))
        metadata["thread_ids"] = thread_ids
        metadata["host_traces"] = []
        metadata["native_spawn_requested"] = False
        metadata["native_spawn_observed"] = False
        for thread_id in thread_ids:
            for trace in (codex_root/"sessions").rglob(f"*{thread_id}*.jsonl"):
                external = project / trace.name
                shutil.copyfile(trace, external)
                destination = evidence / "activity-excerpt.jsonl"
                retained = []
                spawn_call_ids = set()
                for line in trace.read_text().splitlines():
                    item = json.loads(line)
                    payload = item.get("payload", {})
                    if payload.get("type") in ("function_call", "custom_tool_call") and payload.get("name", "").split(".")[-1] == "spawn_agent":
                        try:
                            arguments = json.loads(payload.get("arguments", payload.get("input", "{}")))
                        except (ValueError, TypeError):
                            arguments = {}
                        if arguments.get("agent_type") == "arclint-domain-reviewer":
                            metadata["native_spawn_requested"] = True
                            spawn_call_ids.add(payload.get("call_id"))
                    if payload.get("type") in ("function_call_output", "custom_tool_call_output") and payload.get("call_id") in spawn_call_ids:
                        try:
                            result = json.loads(payload.get("output", "{}"))
                        except (ValueError, TypeError):
                            result = {}
                        if isinstance(result, dict) and result.get("agent_id"):
                            metadata["native_spawn_observed"] = True
                    if item["type"] == "response_item" and (payload.get("type") in
                        ("function_call", "function_call_output", "custom_tool_call", "custom_tool_call_output")
                        or (payload.get("type") == "message" and payload.get("role") == "assistant")):
                        retained.append(line)
                destination.write_text("\n".join(retained)+"\n")
                metadata["host_traces"].append({"source": str(trace), "external_copy": str(external),
                    "copy": destination.name, "sha256": digest(trace), "excerpt_sha256": digest(destination),
                    "excerpt_policy": "All assistant messages and all tool calls/results; private host context omitted"})
    if "execution_status" not in metadata:
        invocation_ok = metadata.get("invocation", {}).get("exit_code") == 0
        response_produced = (evidence/"response.md").exists() and bool((evidence/"response.md").read_text().strip())
        if direct_instructions:
            metadata["execution_status"] = "authored-response-produced" if invocation_ok and response_produced else "authored-response-missing"
        else:
            metadata["execution_status"] = "native-spawn-observed" if metadata.get("native_spawn_observed") else "native-spawn-unverified"
    metadata["behavior_assessment"] = "manual assessment required; process success is not semantic approval"
    metadata["authored_sha256_after"] = digest(ASSET)
    (evidence/"manifest.json").write_text(json.dumps(metadata, indent=2)+"\n")
    print(json.dumps(metadata, indent=2))
    if metadata["execution_status"] not in ("authored-response-produced", "native-spawn-observed"):
        sys.exit(2)

if __name__ == "__main__":
    main()
