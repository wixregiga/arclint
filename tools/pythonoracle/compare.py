#!/usr/bin/env python3
"""Compare every resolved Python fact with pinned Pyright's LSP definitions.

Uses only the Python standard library and npx. Repositories are never edited.
No absent definition, parser error, or mismatching endpoint counts as agreement.
"""
import argparse
import ast
import collections
import json
import os
from pathlib import Path
import queue
import subprocess
import sys
import threading
from urllib.parse import unquote, urlparse

PYRIGHT = "1.1.407"


class LSP:
    def __init__(self, root, extra_paths, python_path):
        self.proc = subprocess.Popen(
            ["npx", "-y", "-p", f"pyright@{PYRIGHT}", "pyright-langserver", "--stdio"],
            stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=sys.stderr,
        )
        self.responses = queue.Queue()
        self.serial = 0
        self.opened = set()
        self.external_cache = {}
        self.root = root
        threading.Thread(target=self.read, daemon=True).start()
        self.request("initialize", {"processId": os.getpid(), "rootUri": root.as_uri(),
                     "capabilities": {}, "workspaceFolders": [{"uri": root.as_uri(), "name": root.name}]})
        self.send("initialized", {})
        self.send("workspace/didChangeConfiguration", {"settings": {"python": {"pythonPath": python_path or sys.executable, "analysis": {
            "autoSearchPaths": True, "extraPaths": extra_paths,
            "diagnosticMode": "openFilesOnly", "useLibraryCodeForTypes": True,
        }}}})

    def read(self):
        try:
            while True:
                headers = {}
                while True:
                    line = self.proc.stdout.readline()
                    if not line:
                        raise EOFError("Pyright closed stdout")
                    if line in (b"\r\n", b"\n"):
                        break
                    key, value = line.decode().split(":", 1)
                    headers[key.lower()] = value.strip()
                msg = json.loads(self.proc.stdout.read(int(headers["content-length"])))
                if "id" in msg and "method" in msg:
                    # Servers may request configuration or capability registration.
                    result = [{} for _ in msg.get("params", {}).get("items", [])] if msg["method"] == "workspace/configuration" else None
                    self.write({"jsonrpc": "2.0", "id": msg["id"], "result": result})
                elif "id" in msg:
                    self.responses.put(msg)
        except Exception as exc:
            self.responses.put(exc)

    def write(self, msg):
        data = json.dumps(msg).encode()
        self.proc.stdin.write(f"Content-Length: {len(data)}\r\n\r\n".encode() + data)
        self.proc.stdin.flush()

    def send(self, method, params):
        self.write({"jsonrpc": "2.0", "method": method, "params": params})

    def request(self, method, params):
        self.serial += 1
        serial = self.serial
        self.write({"jsonrpc": "2.0", "id": serial, "method": method, "params": params})
        while True:
            response = self.responses.get(timeout=60)
            if isinstance(response, Exception):
                raise response
            if response.get("id") == serial:
                if "error" in response:
                    raise RuntimeError(response["error"])
                return response.get("result")

    def query(self, path, method, line, character):
        uri = path.as_uri()
        if uri not in self.opened:
            self.send("textDocument/didOpen", {"textDocument": {"uri": uri, "languageId": "python", "version": 1, "text": path.read_text()}})
            self.opened.add(uri)
        return self.request(method, {"textDocument": {"uri": uri}, "position": {"line": line, "character": character}}) or []

    def external_definitions(self, ref):
        key = (ref["Package"], ref["Owner"], ref["Name"])
        if key in self.external_cache:
            return self.external_cache[key]
        module = ref["Package"] or "builtins"
        imported = ref["Owner"] or ref["Name"]
        expression = "__oracle_symbol" + ("." + ref["Name"] if ref["Owner"] else "")
        text = f"from {module} import {imported} as __oracle_symbol\n{expression}\n"
        uri = (self.root / f".__arclint_oracle_probe_{len(self.external_cache)}.py").as_uri()
        self.send("textDocument/didOpen", {"textDocument": {"uri": uri, "languageId": "python", "version": 1, "text": text}})
        definitions = self.request("textDocument/definition", {"textDocument": {"uri": uri}, "position": {"line": 1, "character": len(expression) - 1}}) or []
        self.send("textDocument/didClose", {"textDocument": {"uri": uri}})
        self.external_cache[key] = definitions
        return definitions

    def close(self):
        try:
            self.request("shutdown", None)
            self.send("exit", None)
            self.proc.wait(timeout=10)
        finally:
            if self.proc.poll() is None:
                self.proc.kill()


TREE_CACHE = {}


def endpoint(location):
    path = Path(unquote(urlparse(location.get("uri", location.get("targetUri"))).path)).resolve()
    pos = location.get("range", location.get("targetSelectionRange"))["start"]
    if path not in TREE_CACHE:
        text = path.read_text()
        TREE_CACHE[path] = ast.parse(text), text.splitlines()
    tree, lines = TREE_CACHE[path]
    row, col = pos["line"] + 1, pos["character"]
    line = lines[row - 1]
    # LSP positions are UTF-16, while AST columns are UTF-8 bytes.
    prefix = line.encode("utf-16-le")[:col * 2].decode("utf-16-le")
    col = len(prefix.encode())
    candidates = []

    def walk(node, owner=""):
        for child in ast.iter_child_nodes(node):
            if isinstance(child, (ast.ClassDef, ast.FunctionDef, ast.AsyncFunctionDef)):
                if child.lineno <= row <= child.end_lineno:
                    if row == child.lineno:
                        candidates.append((child.name, owner))
                    walk(child, child.name if isinstance(child, ast.ClassDef) else owner)
            elif isinstance(child, ast.Name) and child.lineno == row and child.col_offset <= col < child.end_col_offset:
                candidates.append((child.id, owner))
            else:
                walk(child, owner)
    walk(tree)
    if not candidates:
        import re
        words = list(re.finditer(r"[A-Za-z_][A-Za-z_0-9]*", line))
        candidates = [(m.group(), "") for m in words if m.start() <= len(prefix) < m.end()]
    return path, candidates


def compare(root, queries, extra_paths, python_path):
    lsp = LSP(root, extra_paths, python_path)
    counts = collections.Counter()
    reasons = collections.Counter()
    disagreements = []
    try:
        for query in queries:
            ref = query["Expected"]
            if ref["Resolution"] == "undetermined":
                counts["undetermined"] += 1
                reasons[ref["Reason"]] += 1
                continue
            counts["checked"] += 1
            definitions = lsp.query(root / query["File"], query["Method"], query["Line"], query["Character"])
            actual = []
            agreed = False
            probes = lsp.external_definitions(ref) if ref["Resolution"] == "external" else []
            canonical = {(p.get("uri", p.get("targetUri")), json.dumps(p.get("range", p.get("targetSelectionRange"))["start"], sort_keys=True)) for p in probes}
            for definition in definitions:
                try:
                    target, names = endpoint(definition)
                    actual.append({"file": str(target), "names": names})
                    if ref["Resolution"] == "declared":
                        same_symbol = (ref["Name"], ref["Owner"]) in names
                        # Pyright typeDefinition on implicit self points at
                        # its owning method rather than the class header.
                        implicit_self = query["Kind"] == "receiver" and any(owner == ref["Name"] for _, owner in names)
                        agreed |= target == (root / ref["File"]).resolve() and (same_symbol or implicit_self)
                    else:
                        # External Package is the import-facing module (os
                        # re-exports posix); Pyright's endpoint can be its
                        # canonical typeshed module. Require the same member
                        # and owner, and an endpoint outside the repository.
                        location_key = (definition.get("uri", definition.get("targetUri")), json.dumps(definition.get("range", definition.get("targetSelectionRange"))["start"], sort_keys=True))
                        agreed |= not target.is_relative_to(root) and (ref["Name"], ref["Owner"]) in names and location_key in canonical
                except (SyntaxError, ValueError, OSError) as exc:
                    actual.append({"error": str(exc)})
            if agreed:
                counts["agreed"] += 1
                counts[query["Kind"] + "_agreed"] += 1
            else:
                counts["disagreed"] += 1
                disagreements.append({"query": query, "actual": actual, "package_probe": probes})
    finally:
        lsp.close()
    return {"pyright": PYRIGHT, "root": str(root), "counts": dict(counts), "undetermined_reasons": dict(reasons), "disagreements": disagreements}


def main():
    parser = argparse.ArgumentParser(__doc__)
    parser.add_argument("root", type=Path)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--extra-path", action="append", default=[])
    parser.add_argument("--python-path")
    args = parser.parse_args()
    root = args.root.resolve()
    export = args.output.with_suffix(".queries.json").resolve()
    env = dict(os.environ, ARCLINT_PY_ORACLE_ROOT=str(root), ARCLINT_PY_ORACLE_OUTPUT=str(export))
    subprocess.run(["go", "test", "-tags", "pyoracle", "-count=1", "-run", "^TestExportPythonOracle$", "./internal/infrastructure/language/python"], env=env, check=True)
    result = compare(root, json.loads(export.read_text()), args.extra_path, args.python_path)
    args.output.write_text(json.dumps(result, indent=2) + "\n")
    print(json.dumps({"root": str(root), **result["counts"]}))
    return bool(result["disagreements"])


if __name__ == "__main__":
    sys.exit(main())
