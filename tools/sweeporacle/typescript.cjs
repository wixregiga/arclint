// Prints the line sources the TypeScript parser implies for each file named
// on standard input, one "path:line<TAB>source" per line, by the rules the
// domain-sweep inventory documents: comments, literal text that spans lines
// or holds two words, and test calls (it, test, describe, with .each tables,
// type arguments and tagged tables) whose first argument is a literal. The
// inventory's TypeScript lexer is compared with it; see README.md.
"use strict";

const ts = require(process.env.TYPESCRIPT || "typescript");
const fs = require("fs");

const WORDS = /[A-Za-z]{2,}[ \t]+[A-Za-z]{2,}/;
const PRECEDENCE = ["test", "comment", "text"];

function scriptKind(path) {
  if (/\.[cm]?jsx$/.test(path)) return ts.ScriptKind.JSX;
  if (/\.[cm]?js$/.test(path)) return ts.ScriptKind.JS;
  if (/\.tsx$/.test(path)) return ts.ScriptKind.TSX;
  return ts.ScriptKind.TS;
}

function isTestCallee(e) {
  for (;;) {
    if (ts.isIdentifier(e)) return ["it", "test", "describe"].includes(e.text);
    if (ts.isPropertyAccessExpression(e)) e = e.expression;
    else if (ts.isCallExpression(e) && ts.isPropertyAccessExpression(e.expression)) e = e.expression;
    else if (ts.isTaggedTemplateExpression(e)) e = e.tag;
    else return false;
  }
}

function classify(path) {
  const text = fs.readFileSync(path, "utf8");
  const sf = ts.createSourceFile(path, text, ts.ScriptTarget.Latest, true, scriptKind(path));
  const kinds = new Map();
  const line = (pos) => sf.getLineAndCharacterOfPosition(pos).line + 1;
  const add = (n, kind) => {
    if (!kinds.has(n)) kinds.set(n, new Set());
    kinds.get(n).add(kind);
  };
  const seen = new Set();
  const comments = (pos) => {
    for (const ranges of [ts.getLeadingCommentRanges(text, pos), ts.getTrailingCommentRanges(text, pos)]) {
      for (const r of ranges || []) {
        if (seen.has(r.pos)) continue;
        seen.add(r.pos);
        for (let n = line(r.pos); n <= line(r.end); n++) add(n, "comment");
      }
    }
  };
  const literal = (start, end, body) => {
    const first = line(start);
    const last = line(end);
    if (last > first) for (let n = first; n <= last; n++) add(n, "text");
    else if (WORDS.test(body)) add(first, "text");
  };
  const visit = (node) => {
    comments(node.pos);
    const k = node.kind;
    if ((k >= ts.SyntaxKind.FirstToken && k <= ts.SyntaxKind.LastToken) || k === ts.SyntaxKind.EndOfFileToken) {
      comments(node.end);
    }
    const s = node.getStart(sf);
    const e = node.end;
    if (k === ts.SyntaxKind.StringLiteral || k === ts.SyntaxKind.NoSubstitutionTemplateLiteral) {
      literal(s, e, text.slice(s + 1, e - 1));
    } else if (k === ts.SyntaxKind.TemplateHead || k === ts.SyntaxKind.TemplateMiddle) {
      literal(s, e, text.slice(s + 1, e - 2));
    } else if (k === ts.SyntaxKind.TemplateTail) {
      literal(s, e, text.slice(s + 1, e - 1));
    }
    if (ts.isCallExpression(node) && isTestCallee(node.expression) && node.arguments.length) {
      const name = node.arguments[0];
      if (ts.isStringLiteral(name) || ts.isNoSubstitutionTemplateLiteral(name) || ts.isTemplateExpression(name)) {
        add(line(node.expression.getStart(sf)), "test");
        for (let n = line(name.getStart(sf)); n <= line(name.end); n++) add(n, "test");
      }
    }
    node.getChildren(sf).forEach(visit);
  };
  visit(sf);
  const lines = text.split("\n");
  const out = [];
  for (const [n, set] of [...kinds].sort((a, b) => a[0] - b[0])) {
    if (!lines[n - 1] || !lines[n - 1].trim()) continue;
    const kind = PRECEDENCE.find((p) => set.has(p));
    out.push(`${path}:${n}\t${kind}`);
  }
  return out;
}

const paths = fs.readFileSync(0, "utf8").split("\n").filter(Boolean);
const out = [];
for (const path of paths) out.push(...classify(path));
process.stdout.write(out.length ? out.join("\n") + "\n" : "");
