import { test } from "node:test";
import assert from "node:assert/strict";
import { mkdtempSync, writeFileSync, mkdirSync, readFileSync, readdirSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createGuard, commentLines, goComments } from "./guard.mjs";

function harness(t, reviewer = () => ({ relevant: true, findings: [] }), options = {}) {
  const root = mkdtempSync(join(tmpdir(), "arclint-guard-test-"));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  mkdirSync(join(root, ".arclint"));
  writeFileSync(join(root, ".arclint/domain-guard.json"), JSON.stringify({version:1, domainFiles:["domain.yaml"]}));
  const write = text => writeFileSync(join(root, "domain.yaml"), text);
  write("version: 1\ncontexts: []\n");
  const handlers = {}, commands = {}, notices = [];
  let calls = 0, aborted = false;
  const ctx = { cwd: root, sessionManager: { getSessionId: () => "test-session" },
    ui: { setStatus(){}, notify(text) { notices.push(text); } }, abort() { aborted = true; },
    runEphemeralTurn: async options => {
      assert.equal(options.tools, false);
      calls++;
      const result = await reviewer(options);
      return { replyText: typeof result === "string" ? result : JSON.stringify(result) };
    }
  };
  createGuard({ on(name, fn) { handlers[name] = fn; }, registerCommand(name, command) { commands[name] = command; } }, new URL("./index.js", import.meta.url), options);
  const emit = (name, event = {}) => handlers[name](event, ctx);
  const answer = text => emit("assistant_message", { message: { role:"assistant", content:[{type:"text",text}] } });
  const state = () => JSON.parse(readFileSync(join(root, ".arclint/cache/domain-guard", readdirSync(join(root, ".arclint/cache/domain-guard"))[0]), "utf8"));
  return {root, ctx, write, emit, answer, state, notices, commands, calls:()=>calls, aborted:()=>aborted};
}

test("comments distinguish quoted hashes, block scalars and apostrophes", () => {
  const source = '# comment\nname: "A # B"\ntext: |\n  # literal\nname2: owner\'s promise # real\nother: \'a # b\'\n';
  assert.deepEqual(commentLines(source).map(x=>x.line), [1,5]);
});

test("loaded host commands are inert and completion requires a fresh response review", async t => {
  const h = harness(t);
  await h.emit("session_start");
  await h.commands["arclint-domain-status"].handler("", h.ctx);
  assert.equal(h.calls(),0);
  assert.equal((await h.emit("session_stop")).decision,"block");
  await h.emit("before_agent_start",{prompt:"Review the domain."});
  await h.answer("The domain has no recorded contexts.");
  assert.equal(await h.emit("session_stop"),undefined);
  assert.equal(h.state().verdict.status,"passed");
});

test("comment violation blocks; scoped repair passes; state deduplicates and resolves", async t => {
  const h = harness(t);
  await h.emit("session_start");
  await h.emit("before_agent_start",{prompt:"Update the domain."});
  h.write("version: 1\n# invented commentary\ncontexts: []\n");
  await h.emit("tool_result");
  await h.answer("Finished.");
  assert.equal((await h.emit("session_stop")).decision,"block");
  await h.answer("Finished.");
  assert.equal(h.state().findings.length,1);
  assert.equal(h.state().findings[0].open,true);
  h.write("version: 1\ncontexts: []\n");
  await h.emit("tool_result");
  await h.answer("Removed the comment.");
  assert.equal(await h.emit("session_stop"),undefined);
  assert.equal(h.state().findings[0].open,false);
});

test("proposal rejects evidenced scope expansion before execution", async t => {
  const h = harness(t, options => options.promptText.includes("Review stage: proposal") ?
    {relevant:true,findings:[{file:"proposal",quote:"NewCategory",reason:"The request asks for a Rule; this adds a category.",repair:"Use the existing Rule."}]} :
    {relevant:true,findings:[]});
  await h.emit("before_agent_start",{prompt:"Use the existing Rule."});
  const result = await h.emit("tool_call",{toolName:"write",input:{path:"domain.yaml",content:"NewCategory"}});
  assert.equal(result.block,true);
  assert.match(result.reason,/existing Rule/);
  assert.equal(readFileSync(join(h.root,"domain.yaml"),"utf8"),"version: 1\ncontexts: []\n");
});

test("an external or shell edit after review makes completion stale", async t => {
  const h = harness(t);
  await h.emit("before_agent_start",{prompt:"Review domain."});
  await h.answer("Reviewed.");
  h.write("version: 2\ncontexts: []\n");
  assert.equal((await h.emit("session_stop")).decision,"block");
});

test("changes during semantic review cannot receive approval", async t => {
  let h;
  h = harness(t, options => {
    if(options.promptText.includes("Review stage: completion")) h.write("version: 2\ncontexts: []\n");
    return {relevant:true,findings:[]};
  });
  await h.emit("before_agent_start",{prompt:"Review domain."});
  await h.answer("Reviewed.");
  assert.match(h.state().error,/stale/);
  assert.equal((await h.emit("session_stop")).decision,"block");
});

test("unavailable host capability never passes, repeated failures pause", async t => {
  const h = harness(t);
  delete h.ctx.runEphemeralTurn;
  await h.emit("before_agent_start",{prompt:"Review domain."});
  await h.answer("Done.");
  for(let i=0;i<3;i++) assert.equal((await h.emit("session_stop")).decision,"block");
  assert.equal(h.aborted(),true);
  assert.equal(h.state().verdict.status,"unavailable");
});

test("malformed and unevidenced reviewer output never passes", async t => {
  for(const output of ["not json",{relevant:true,findings:[{file:"domain.yaml",quote:"invented",reason:"Bad",repair:"Change it"}]}]) {
    const h = harness(t,()=>output);
    await h.emit("before_agent_start",{prompt:"Review domain."});
    await h.answer("Done.");
    assert.equal((await h.emit("session_stop")).decision,"block");
    assert.equal(h.state().verdict.status,"unavailable");
  }
});

test("timeout fails closed within the host handler budget", async t => {
  const h = harness(t,()=>new Promise(()=>{}),{timeoutMs:10});
  await h.emit("before_agent_start",{prompt:"Review domain."});
  assert.match(h.state().error,/timed out/);
  assert.equal((await h.emit("session_stop")).decision,"block");
});

test("configuration weakening after approval is detected", async t => {
  const h = harness(t);
  await h.emit("before_agent_start",{prompt:"Review domain."});
  await h.answer("Done.");
  writeFileSync(join(h.root,".arclint/domain-guard.json"),JSON.stringify({version:1,domainFiles:[]}));
  assert.equal((await h.emit("session_stop")).decision,"block");
});

test("findings about irrelevant response reach the builder", async t => {
  const h = harness(t,options => options.promptText.includes("Review stage: completion") ?
    {relevant:true,findings:[{file:"response",quote:"I added a new taxonomy",reason:"The request only asks for the existing Rule.",repair:"Implement the requested Rule and report that result."}]} :
    {relevant:true,findings:[]});
  await h.emit("before_agent_start",{prompt:"Report missing contracts using a Rule."});
  await h.answer("I added a new taxonomy");
  const stop = await h.emit("session_stop");
  assert.equal(stop.decision,"block");
  assert.match(stop.reason,/existing Rule/);
});

test("changes outside configured domain do not invalidate a passing verdict", async t => {
  const h = harness(t);
  await h.emit("before_agent_start",{prompt:"Review domain."});
  await h.answer("Reviewed.");
  writeFileSync(join(h.root,"unrelated.txt"),"unrelated");
  assert.equal(await h.emit("session_stop"),undefined);
});

test("stop continuations neither reset retry limit nor rewrite the user request", async t => {
  const h = harness(t);
  delete h.ctx.runEphemeralTurn;
  await h.emit("input",{source:"rpc",text:"Original requirement"});
  for(let i=0;i<3;i++) {
    await h.emit("before_agent_start",{prompt:i ? "Internal repair reminder" : "Original requirement"});
    await h.answer("Done.");
    assert.equal((await h.emit("session_stop")).decision,"block");
  }
  assert.equal(h.aborted(),true);
  assert.deepEqual(h.state().requests,["Original requirement"]);
});

test("explicit source patterns are reviewed, source edits invalidate completion", async t => {
  const h = harness(t);
  mkdirSync(join(h.root,"src"));
  writeFileSync(join(h.root,"src/domain.go"),"package domain\n");
  writeFileSync(join(h.root,".arclint/domain-guard.json"),JSON.stringify({version:1,domainFiles:["domain.yaml"],sourcePatterns:["src/**/*.go"]}));
  await h.emit("before_agent_start",{prompt:"Review domain implementation."});
  await h.answer("Reviewed.");
  assert.equal(await h.emit("session_stop"),undefined);
  writeFileSync(join(h.root,"src/domain.go"),"package changed\n");
  assert.equal((await h.emit("session_stop")).decision,"block");
});

test("explicit source path receives pre-edit review even without YAML edit", async t => {
  const h = harness(t, options => options.promptText.includes("Review stage: proposal") ?
    {relevant:true,findings:[{file:"proposal",quote:"NewEvaluator",reason:"The request requires an existing Rule.",repair:"Reuse Rule."}]} :
    {relevant:false,findings:[]});
  mkdirSync(join(h.root,"src"));
  writeFileSync(join(h.root,"src/domain.go"),"package domain\n");
  writeFileSync(join(h.root,".arclint/domain-guard.json"),JSON.stringify({version:1,domainFiles:["domain.yaml"],sourcePatterns:["src/*.go"]}));
  await h.emit("before_agent_start",{prompt:"Report missing contracts."});
  const result=await h.emit("tool_call",{toolName:"write",input:{path:"src/domain.go",content:"NewEvaluator"}});
  assert.equal(result.block,true);
});

test("rules and baseline changes invalidate approval", async t => {
  for(const path of ["rules.arclint.yaml",".arclint/baseline.v2.json"]) {
    const h=harness(t);
    await h.emit("before_agent_start",{prompt:"Review domain."});
    await h.answer("Reviewed.");
    writeFileSync(join(h.root,path),"changed");
    assert.equal((await h.emit("session_stop")).decision,"block");
  }
});

test("empty source pattern fails closed", async t => {
  const h=harness(t);
  mkdirSync(join(h.root,"src"));
  writeFileSync(join(h.root,".arclint/domain-guard.json"),JSON.stringify({version:1,domainFiles:["domain.yaml"],sourcePatterns:["src/*.go"]}));
  await h.emit("before_agent_start",{prompt:"Review domain."});
  await h.answer("Done.");
  assert.equal((await h.emit("session_stop")).decision,"block");
});

test("Go lexer ignores URL/string/rune/raw literals and reports real comment lines", () => {
  const source = 'package domain\nvar url = "https://host/*path*/"\nvar raw = \x60// literal\n/* literal */\x60\nvar rune = \'/\'\n// explanation\nvar n = 1 /* inline */\n';
  const found = goComments(source);
  assert.deepEqual(found.map(x=>x.quote), ["// explanation", "/* inline */"]);
  assert.deepEqual(found.map(x=>x.line), [6,7]);
  assert.deepEqual(goComments('package domain\nvar s = "quote: \\" // still string"\n'), []);
});

test("Go policy preserves required directives, legal headers, generated markers and cgo", () => {
  const source = '// Copyright 2026 Example\n// Licensed under the Apache License.\n\n// SPDX-License-Identifier: MIT\n\n//go:build linux\n// +build linux\n\n// Code generated by example. DO NOT EDIT.\n\npackage domain\n//go:embed template.txt\nvar template string\n//line domain.go:1\n/* #cgo CFLAGS: -DVALUE=1\n#include <stdint.h> */\nimport "C"\n';
  assert.ok(goComments(source).every(x=>x.preservedBy));
  assert.equal(goComments("package domain\n// Copyright is a business term\n")[0].preservedBy,null);
});

test("domain-source comments block and repair while contextual source comments remain", async t => {
  const h = harness(t);
  mkdirSync(join(h.root,"src"));
  writeFileSync(join(h.root,"src/domain.go"),"package domain\n");
  writeFileSync(join(h.root,"src/context.go"),"package transport\n// HTTP adapter explanation\n");
  writeFileSync(join(h.root,".arclint/domain-guard.json"),JSON.stringify({version:1,domainFiles:["domain.yaml"],sourcePatterns:["src/context.go"],domainSourcePatterns:["src/domain.go"]}));
  await h.emit("session_start");
  await h.emit("before_agent_start",{prompt:"Update the domain"});
  writeFileSync(join(h.root,"src/domain.go"),"package domain\n// unwanted explanation\n");
  await h.emit("tool_result");
  await h.answer("Done");
  assert.equal((await h.emit("session_stop")).decision,"block");
  assert.ok(h.state().findings.some(x=>x.file==="src/domain.go"));
  writeFileSync(join(h.root,"src/domain.go"),'package domain\nvar endpoint = "https://example.test"\n');
  await h.emit("tool_result");
  await h.answer("Repaired");
  assert.equal(await h.emit("session_stop"),undefined);
  assert.match(readFileSync(join(h.root,"src/context.go"),"utf8"),/HTTP adapter/);
  writeFileSync(join(h.root,"src/domain.go"),"package changed\n");
  assert.equal((await h.emit("session_stop")).decision,"block");
});

test("unsupported domain-source languages fail unavailable instead of silently passing", async t => {
  const h = harness(t);
  mkdirSync(join(h.root,"src"));
  writeFileSync(join(h.root,"src/domain.ts"),"const x = 1;");
  writeFileSync(join(h.root,".arclint/domain-guard.json"),JSON.stringify({version:1,domainFiles:["domain.yaml"],domainSourcePatterns:["src/domain.ts"]}));
  await h.emit("session_start");
  assert.ok(h.notices.some(text => /support .go only/.test(text)));
  assert.equal((await h.emit("session_stop")).decision,"block");
});

test("unterminated Go literals and comments cannot pass review", () => {
  for (const source of ['package domain\n/* missing','package domain\nvar s = "missing']) assert.throws(()=>goComments(source),/Unterminated Go/);
});

test("directives survive adjacent explanation and line cgo", () => {
  const source = 'package domain\n//go:embed file.txt\n// explanation\nvar text string\n/*line generated.go:2*/\n// #include <stdio.h>\nimport "C"\n';
  assert.deepEqual(goComments(source).filter(x => !x.preservedBy).map(x => x.quote), ["// explanation"]);
  assert.deepEqual(commentLines("# yaml-language-server: $schema=domain.json\n# explanation\n").map(x => x.line), [2]);
});
