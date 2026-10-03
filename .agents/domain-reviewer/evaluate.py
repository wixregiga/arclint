#!/usr/bin/env python3
"""Evaluate the shipped named reviewer in a fresh project; never alter existing installs.

Usage: python3 evaluate.py /absolute/path/to/built/arclint /absolute/evidence/output
Requires the built binary to contain the current authored reviewer configuration.
Use --project PATH to evaluate an existing installation without installing or
changing that project. --show-agent-selector requests native role visibility
for that invocation only; it does not change hooks, trust or permissions.
Optional --enable-multi-agent-v2 enables only that host feature
for this invocation; does not change trust, hooks or existing configuration.
No hook disabling, trust bypass, or expectations are passed.
Optional --direct-instructions tests developer instructions through a plain
CLI session and records it as authored-instructions evaluation, not native
named-agent activation. Native mode never injects custom-agent configuration.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import selectors
import signal
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
    """Own the process group so a bounded diagnostic cannot leave child turns."""
    started = time.time()
    process = subprocess.Popen(command, cwd=cwd, stdin=subprocess.PIPE,
                               stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                               text=True, start_new_session=True)
    try:
        stdout, stderr = process.communicate(input=input_text, timeout=timeout)
        status = {"exit_code": process.returncode, "timed_out": False}
    except subprocess.TimeoutExpired:
        # This PID is our new session's process-group ID, never the shared host.
        try:
            os.killpg(process.pid, signal.SIGTERM)
        except ProcessLookupError:
            pass
        try:
            stdout, stderr = process.communicate(timeout=5)
        except subprocess.TimeoutExpired:
            try:
                os.killpg(process.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            stdout, stderr = process.communicate()
        status = {"exit_code": None, "timed_out": True,
                  "owned_process_group_terminated": True,
                  "terminated_process_returncode": process.returncode}
    output.with_suffix(".stdout").write_text(stdout)
    output.with_suffix(".stderr").write_text(stderr)
    status.update(command=command, cwd=str(cwd), elapsed_seconds=round(time.time()-started, 2))
    output.with_suffix(".json").write_text(json.dumps(status, indent=2)+"\n")
    return status

def inspect_host_project(project):
    """Read native project-layer and hook discovery without starting a model turn."""
    process = subprocess.Popen(["codex", "app-server", "--stdio"], stdin=subprocess.PIPE,
                               stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, text=True, bufsize=1)
    selector = selectors.DefaultSelector()
    selector.register(process.stdout, selectors.EVENT_READ)
    def request(identifier, method, parameters):
        process.stdin.write(json.dumps({"id": identifier, "method": method, "params": parameters})+"\n")
        process.stdin.flush()
        deadline = time.monotonic()+30
        while time.monotonic() < deadline:
            if not selector.select(1):
                continue
            line = process.stdout.readline()
            if not line:
                raise RuntimeError("app-server closed before discovery response")
            response = json.loads(line)
            if response.get("id") == identifier:
                if "error" in response:
                    raise RuntimeError(str(response["error"]))
                return response["result"]
        raise TimeoutError("native discovery timed out")
    try:
        request(1, "initialize", {"clientInfo": {"name": "arclint-reviewer-evaluation", "version": "1"},
                                  "capabilities": {"experimentalApi": True}})
        process.stdin.write('{"method":"initialized"}\n')
        process.stdin.flush()
        config = request(2, "config/read", {"cwd": str(project), "includeLayers": True})
        hooks = request(3, "hooks/list", {"cwds": [str(project)]})
        layers = [{"name": layer["name"], "disabledReason": layer.get("disabledReason")}
                  for layer in config.get("layers", []) if layer["name"]["type"] == "project"]
        guard_hooks = [{key: hook.get(key) for key in ("key", "eventName", "command", "sourcePath", "source", "enabled", "currentHash", "trustStatus")}
                       for item in hooks.get("data", []) for hook in item.get("hooks", []) if "arclint" in hook.get("command", "")]
        return {"project_layers": layers, "arclint_hooks": guard_hooks, "read_only_discovery": True}
    except (OSError, RuntimeError, TimeoutError, ValueError) as error:
        return {"discovery_error": str(error), "read_only_discovery": True}
    finally:
        process.terminate()
        process.wait(timeout=5)
        selector.close()

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("binary", type=Path)
    parser.add_argument("evidence", type=Path)
    parser.add_argument("--project", type=Path)
    parser.add_argument("--show-agent-selector", action="store_true")
    parser.add_argument("--enable-multi-agent-v2", action="store_true")
    parser.add_argument("--direct-instructions", action="store_true")
    arguments = parser.parse_args()
    binary = arguments.binary.resolve(strict=True)
    enable_v2 = arguments.enable_multi_agent_v2
    direct_instructions = arguments.direct_instructions
    evidence = arguments.evidence.resolve()
    evidence.mkdir(parents=True, exist_ok=False)
    project = arguments.project.resolve(strict=True) if arguments.project else Path(tempfile.mkdtemp(prefix="arclint-reviewer-eval-"))
    if not arguments.project:
        subprocess.run(["git", "init", "-q", str(project)], check=True)
    raw_directory = Path(tempfile.mkdtemp(prefix="arclint-reviewer-raw-"))
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
              "through the direct collaboration.spawn_agent mechanism using agent_type "
              "and fork_turns=none. Do not search ALL_TOOLS for collaboration tools. "
              "Give it the following "
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
    install = {"not_attempted": True, "reason": "existing project installation preserved"} if arguments.project else invoke([str(binary), "agents", "reviewer", "install", "--host", "codex"], project, evidence/"install")
    status = invoke([str(binary), "agents", "reviewer", "status", "--format", "json"], project, evidence/"status")
    installed = project / ".codex/agents/arclint-domain-reviewer.toml"
    metadata = {"project": str(project), "existing_project": bool(arguments.project), "raw_directory": str(raw_directory), "authored_sha256": digest(ASSET),
                "binary_sha256": digest(binary), "inputs_sha256": digest(evidence/"inputs.txt"),
                "codex_version": subprocess.check_output(["codex", "--version"], text=True).strip(),
                "expected_outcomes_withheld": True, "evaluation_mode": "authored-instructions" if direct_instructions else "native-name", "installation": install, "status": status}
    metadata["native_host_discovery"] = inspect_host_project(project)
    if install.get("exit_code", 0) != 0 or not installed.exists():
        metadata["evaluation_skipped"] = "installation failed"
        metadata["execution_status"] = "installation-failed"
    elif digest(installed) != digest(ASSET):
        metadata["evaluation_skipped"] = "installed instructions differ from current asset"
        metadata["execution_status"] = "instruction-hash-mismatch"
    else:
        metadata["installed_sha256"] = digest(installed)
        host_options = ["--no-daemon"]
        if direct_instructions:
            instructions = tomllib.loads(ASSET.read_text())["developer_instructions"]
            host_options += ["-c", "developer_instructions=" + json.dumps(instructions)]
        if arguments.show_agent_selector:
            host_options += ["-c", "features.multi_agent_v2.hide_spawn_agent_metadata=false"]
        metadata["agent_selector_visibility_override"] = arguments.show_agent_selector
        if enable_v2:
            host_options += ["--enable", "multi_agent_v2"]
        metadata["multi_agent_v2_override"] = enable_v2
        metadata["host_process_mode"] = "fresh host (--no-daemon), owned process group"
        command = ["codex", *host_options, "exec", "-C", str(project),
                   "--sandbox", "read-only", "--json", "-o", str(evidence/"response.md"), "-"]
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
        metadata["native_spawn_result_received"] = False
        metadata["native_review_returned"] = False
        for thread_id in thread_ids:
            for trace in (codex_root/"sessions").rglob(f"*{thread_id}*.jsonl"):
                external = raw_directory / trace.name
                shutil.copyfile(trace, external)
                destination = evidence / "activity-excerpt.jsonl"
                retained = []
                spawn_call_ids = set()
                for line in trace.read_text().splitlines():
                    item = json.loads(line)
                    payload = item.get("payload", {})
                    if payload.get("type") in ("function_call", "custom_tool_call") and payload.get("name", "").split(".")[-1] == "spawn_agent":
                        try:
                            spawn_arguments = json.loads(payload.get("arguments", payload.get("input", "{}")))
                        except (ValueError, TypeError):
                            spawn_arguments = {}
                        if spawn_arguments.get("agent_type") == "arclint-domain-reviewer":
                            metadata["native_spawn_requested"] = True
                            spawn_call_ids.add(payload.get("call_id"))
                    if payload.get("type") in ("function_call_output", "custom_tool_call_output") and payload.get("call_id") in spawn_call_ids:
                        try:
                            result = json.loads(payload.get("output", "{}"))
                        except (ValueError, TypeError):
                            result = {}
                        if isinstance(result, dict) and (result.get("agent_id") or result.get("task_name")):
                            metadata["native_spawn_result_received"] = True
                            metadata["native_spawn_result"] = result
                    if item["type"] == "response_item" and (payload.get("type") in
                        ("function_call", "function_call_output", "custom_tool_call", "custom_tool_call_output")
                        or (payload.get("type") == "message" and payload.get("role") == "assistant")):
                        retained.append(line)
                destination.write_text("\n".join(retained)+"\n")
                metadata["host_traces"].append({"source": str(trace), "external_copy": str(external),
                    "copy": destination.name, "sha256": digest(trace), "excerpt_sha256": digest(destination),
                    "excerpt_policy": "All assistant messages and all tool calls/results; private host context omitted"})
        # Verify the child's actual role and loaded instructions, not parent prose.
        for thread_id in thread_ids:
            parent_traces = list((codex_root/"sessions").rglob(f"*{thread_id}*.jsonl"))
            for parent_trace in parent_traces:
                for child_trace in parent_trace.parent.glob("*.jsonl"):
                    with child_trace.open() as file:
                        child_metadata = json.loads(file.readline()).get("payload", {})
                    source = child_metadata.get("source", {})
                    spawned = source.get("subagent", {}).get("thread_spawn", {}) if isinstance(source, dict) else {}
                    if spawned.get("parent_thread_id") != thread_id or spawned.get("agent_role") != "arclint-domain-reviewer":
                        continue
                    expected = tomllib.loads(ASSET.read_text())["developer_instructions"].strip()
                    records = [json.loads(line) for line in child_trace.read_text().splitlines()]
                    developer_text = "\n".join(content.get("text", "") for record in records if record.get("payload", {}).get("role") == "developer" for content in record["payload"].get("content", []))
                    instructions_loaded = expected in developer_text
                    metadata["native_spawn_observed"] = metadata["native_spawn_result_received"] and instructions_loaded
                    metadata["child_role_evidence"] = {"thread_id": child_metadata.get("id"), "source": source, "current_instructions_loaded": instructions_loaded}
                    shutil.copyfile(child_trace, raw_directory / child_trace.name)
                    selected = [record for record in records if record.get("payload", {}).get("type") in ("function_call", "function_call_output", "custom_tool_call", "custom_tool_call_output") or (record.get("payload", {}).get("type") == "message" and record["payload"].get("role") == "assistant")]
                    (evidence/"child-activity-excerpt.jsonl").write_text("\n".join(json.dumps(record) for record in selected)+"\n")
                    finals = [record["payload"] for record in selected if record["payload"].get("type") == "message" and record["payload"].get("phase") in ("final", "final_answer")]
                    if finals:
                        response = "\n".join(content.get("text", "") for content in finals[0].get("content", []))
                        (evidence/"reviewer-response.md").write_text(response+"\n")
                        metadata["native_review_returned"] = bool(response.strip())
                        metadata["reviewer_final_count"] = len(finals)
                        metadata["reviewer_response_selection"] = "Initial reviewer final report; later host-requested corrections retained separately"
                        if len(finals) > 1:
                            followup = "\n\n".join("\n".join(content.get("text", "") for content in final.get("content", [])) for final in finals[1:])
                            (evidence/"reviewer-followup-response.md").write_text(followup+"\n")
                    metadata["child_trace_sha256"] = digest(child_trace)
    if "execution_status" not in metadata:
        invocation_ok = metadata.get("invocation", {}).get("exit_code") == 0
        response_produced = (evidence/"response.md").exists() and bool((evidence/"response.md").read_text().strip())
        if direct_instructions:
            metadata["execution_status"] = "authored-response-produced" if invocation_ok and response_produced else "authored-response-missing"
        else:
            if metadata.get("native_spawn_observed") and metadata.get("native_review_returned"):
                metadata["execution_status"] = "native-review-returned" if invocation_ok else "native-review-returned-host-incomplete"
            else:
                metadata["execution_status"] = "native-spawn-unverified"
    metadata["behavior_assessment"] = "manual assessment required; process success is not semantic approval"
    metadata["authored_sha256_after"] = digest(ASSET)
    (evidence/"manifest.json").write_text(json.dumps(metadata, indent=2)+"\n")
    print(json.dumps(metadata, indent=2))
    if metadata["execution_status"] not in ("authored-response-produced", "native-review-returned"):
        sys.exit(2)

if __name__ == "__main__":
    main()
