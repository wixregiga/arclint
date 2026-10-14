// Independent oracle: only the compiler's checker supplies declaration
// identities. Adapter results are input, never used to choose a symbol.
const fs = require('node:fs');
const path = require('node:path');
const compiler = process.env.PATH.split(path.delimiter).map(p => path.join(p, 'tsc')).find(p => {
  try { return fs.realpathSync(p).includes('typescript/bin/tsc'); } catch { return false; }
});
if (!compiler) throw new Error('Run through npm exec --package=typescript@5.9.3');
const ts = require(path.resolve(fs.realpathSync(compiler), '../../lib/typescript.js'));
if (ts.version !== '5.9.3') throw new Error(`Expected TypeScript 5.9.3, got ${ts.version}`);
const input = JSON.parse(fs.readFileSync(process.argv[2], 'utf8'));
const root = path.resolve(input.Root);
const names = input.Files.map(f => path.join(root, f.Path)).filter(f => /\.tsx?$/.test(f));
const programs = new Map();
function programFor(file) {
  const config = ts.findConfigFile(path.dirname(file), fs.existsSync, 'tsconfig.json');
  const key = config || '';
  if (!programs.has(key)) {
    let options = { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS, moduleResolution: ts.ModuleResolutionKind.Node10, jsx: ts.JsxEmit.ReactJSX, skipLibCheck: true };
    let roots = names;
    if (config) {
      const read = ts.readConfigFile(config, ts.sys.readFile);
      if (read.error) throw new Error(ts.flattenDiagnosticMessageText(read.error.messageText, '\n'));
      const parsed = ts.parseJsonConfigFileContent(read.config, ts.sys, path.dirname(config));
      options = { ...options, ...parsed.options };
      roots = [...new Set([...parsed.fileNames, ...names.filter(n => n.startsWith(path.dirname(config) + path.sep))])];
    }
    const host = ts.createCompilerHost(options);
    host.getCurrentDirectory = () => config ? path.dirname(config) : root;
    programs.set(key, ts.createProgram(roots, options, host));
  }
  return programs.get(key);
}
let checked = 0, agreed = 0, undetermined = 0, unavailable = 0;
const disagreements = [], reasons = {}, samples = [];
function dereference(checker, symbol) {
  const seen = new Set();
  while (symbol && (symbol.flags & ts.SymbolFlags.Alias)) {
    if (seen.has(symbol)) return undefined;
    seen.add(symbol); symbol = checker.getAliasedSymbol(symbol);
  }
  return symbol;
}
function ownerOf(decl) {
  for (let n = decl.parent; n; n = n.parent) {
    if (ts.isClassDeclaration(n) || ts.isInterfaceDeclaration(n) || ts.isTypeAliasDeclaration(n)) return n.name?.text || 'default';
    if (ts.isSourceFile(n)) break;
  }
  return '';
}
function identity(decl, symbol) {
  if (!decl) return undefined;
  const file = path.resolve(decl.getSourceFile().fileName);
  const name = decl.name?.text || symbol?.getName();
  return { file: path.relative(root, file).split(path.sep).join('/'), owner: ownerOf(decl), name, absolute: file };
}
function symbolIdentities(checker, symbol) {
  symbol = dereference(checker, symbol);
  return (symbol?.getDeclarations() || []).map(d => identity(d, symbol)).filter(Boolean);
}
function moduleOrigin(checker, node, namespaceMember) {
  if (ts.isQualifiedName(node)) return moduleOrigin(checker, node.left, node.right.text);
  if (ts.isPropertyAccessExpression(node) && namespaceBinding(checker,node.expression)) return moduleOrigin(checker,node.expression,node.name.text);
  const symbol = checker.getSymbolAtLocation(node);
  return bindingOrigin(checker, symbol, namespaceMember, new Set());
}
function outsideRepository(identity) {
  return !!identity.builtin || !!identity.absolute?.includes('/node_modules/') || !!identity.file?.startsWith('../');
}
function bindingOrigin(checker, symbol, namespaceMember, seen) {
  if (!symbol || seen.has(symbol)) return undefined;
  seen.add(symbol);
  for (const decl of symbol.getDeclarations() || []) {
    let statement = decl;
    while (statement && !ts.isImportDeclaration(statement) && !ts.isExportDeclaration(statement)) statement = statement.parent;
    if (!statement) continue;
    const isNamespace = ts.isNamespaceImport(decl);
    const imported = ts.isImportSpecifier(decl) || ts.isExportSpecifier(decl) ? (decl.propertyName || decl.name).text : isNamespace ? (namespaceMember || '*') : 'default';
    if (statement.moduleSpecifier && ts.isStringLiteral(statement.moduleSpecifier)) {
      const module = checker.getSymbolAtLocation(statement.moduleSpecifier);
      return exportedOrigin(checker, module, statement.moduleSpecifier.text, imported, symbolIdentities(checker, symbol), seen);
    }
    if (ts.isExportSpecifier(decl)) {
      const local = checker.getExportSpecifierLocalTargetSymbol(decl);
      const origin = bindingOrigin(checker, local, namespaceMember, seen);
      if (origin) return origin;
    }
  }
  return undefined;
}
function exportedOrigin(checker, module, spec, name, ids, seen) {
  if (!module) return undefined;
  const moduleIds = symbolIdentities(checker, module);
  if (moduleIds.length && moduleIds.every(outsideRepository) && ids.length && ids.every(outsideRepository)) return { pkg: spec, imported: name, ids };
  const exported = checker.getExportsOfModule(module).find(s => s.getName() === name);
  if (!exported) return undefined;
  const direct = bindingOrigin(checker, exported, undefined, seen);
  if (direct) return direct;
  const reached = dereference(checker, exported);
  for (const decl of module.getDeclarations() || []) {
    const source = decl.getSourceFile();
    for (const statement of source.statements) {
      if (!ts.isExportDeclaration(statement) || statement.exportClause || !statement.moduleSpecifier) continue;
      const next = checker.getSymbolAtLocation(statement.moduleSpecifier);
      if (!next) continue;
      const candidate = checker.getExportsOfModule(next).find(s => s.getName() === name);
      if (candidate && dereference(checker, candidate) === reached) {
        const key = statement;
        if (seen.has(key)) continue;
        seen.add(key);
        const origin = exportedOrigin(checker, next, statement.moduleSpecifier.text, name, symbolIdentities(checker, candidate), seen);
        if (origin) return origin;
      }
    }
  }
  return undefined;
}
function namespaceBinding(checker, node) {
  return (checker.getSymbolAtLocation(node)?.getDeclarations() || []).some(ts.isNamespaceImport);
}
function actualCall(checker, call) {
  let fn = ts.isTaggedTemplateExpression(call) ? call.tag : call.expression;
  if (ts.isPropertyAccessExpression(fn)) {
    const ids = symbolIdentities(checker, checker.getSymbolAtLocation(fn.name));
    const annotation = annotationOfValue(checker, fn.expression);
    const origin = moduleOrigin(checker, fn.expression, namespaceBinding(checker,fn.expression) ? fn.name.text : undefined) || valueTypeOrigin(checker,fn.expression) || (annotation && ts.isTypeReferenceNode(annotation) ? moduleOrigin(checker, annotation.typeName) : undefined);
    return { ids, origin, member: fn.name.text, namespace: namespaceBinding(checker,fn.expression) };
  }
  const ids = symbolIdentities(checker, checker.getSymbolAtLocation(fn));
  return { ids, origin: moduleOrigin(checker, fn) };
}
function annotationOfValue(checker, n) {
  if (ts.isParenthesizedExpression(n)) return annotationOfValue(checker, n.expression);
  const symbol = checker.getSymbolAtLocation(ts.isPropertyAccessExpression(n) ? n.name : n);
  const declarations = dereference(checker, symbol)?.getDeclarations() || [];
  for (const d of declarations) if (d.type) return d.type;
  return undefined;
}
function typeIdentities(checker, typeNode, seen = new Set()) {
  if (!typeNode || seen.has(typeNode)) return [];
  seen.add(typeNode);
  if (ts.isFunctionTypeNode(typeNode) || ts.isConstructorTypeNode(typeNode)) return [];
  const result = [];
  if (ts.isTypeReferenceNode(typeNode)) {
    const symbol = dereference(checker, checker.getSymbolAtLocation(typeNode.typeName));
    result.push(...symbolIdentities(checker, symbol));
    for (const decl of symbol?.getDeclarations() || []) if (ts.isTypeAliasDeclaration(decl)) result.push(...typeIdentities(checker, decl.type, seen));
    for (const arg of typeNode.typeArguments || []) result.push(...typeIdentities(checker, arg, seen));
  } else if (ts.isArrayTypeNode(typeNode) || ts.isTupleTypeNode(typeNode)) {
    const array = checker.resolveName(ts.isTypeOperatorNode(typeNode.parent) && typeNode.parent.operator === ts.SyntaxKind.ReadonlyKeyword ? 'ReadonlyArray' : 'Array', typeNode, ts.SymbolFlags.Type, false);
    result.push(...symbolIdentities(checker, array));
    if (ts.isArrayTypeNode(typeNode)) result.push(...typeIdentities(checker, typeNode.elementType, seen));
    else for (const e of typeNode.elements) result.push(...typeIdentities(checker, e, seen));
  } else {
    const type = checker.getTypeFromTypeNode(typeNode);
    result.push(...symbolIdentities(checker, type.aliasSymbol || type.getSymbol()));
    const primitive = primitiveName(type);
    if (primitive) result.push({ builtin: primitive });
    ts.forEachChild(typeNode, child => {
      if (ts.isTypeNode(child)) result.push(...typeIdentities(checker, child, seen));
      else if (ts.isPropertySignature(child) && child.type) result.push(...typeIdentities(checker, child.type, seen));
    });
  }
  return result;
}
function primitiveName(type) {
  const flags = ts.TypeFlags;
  for (const [flag, name] of [['StringLike','string'],['NumberLike','number'],['BooleanLike','boolean'],['BigIntLike','bigint'],['ESSymbolLike','symbol'],['Null','null'],['Undefined','undefined'],['Void','void'],['Never','never'],['NonPrimitive','object']]) {
    if (type.flags & flags[flag]) return name;
  }
}
function valueTypeOrigin(checker, node) {
  const direct = moduleOrigin(checker,node);
  if (direct) return direct;
  const symbol = dereference(checker,checker.getSymbolAtLocation(ts.isPropertyAccessExpression(node) ? node.name : node));
  for (const decl of symbol?.getDeclarations() || []) {
    if (decl.initializer && ts.isNewExpression(decl.initializer)) {
      const origin = moduleOrigin(checker,decl.initializer.expression);
      if (origin) return origin;
    }
  }
  return undefined;
}
function receiverActual(checker, call) {
  const fn = ts.isTaggedTemplateExpression(call) ? call.tag : call.expression;
  if (!ts.isPropertyAccessExpression(fn)) return { ids: [] };
  const base = fn.expression;
  const valueOrigin = valueTypeOrigin(checker, base);
  const annotation = annotationOfValue(checker, base);
  if (annotation) {
    if (ts.isTypeReferenceNode(annotation)) return { ids: symbolIdentities(checker, checker.getSymbolAtLocation(annotation.typeName)), origin: valueOrigin || moduleOrigin(checker, annotation.typeName) };
    return { ids: typeIdentities(checker, annotation), origin: valueOrigin };
  }
  const type = checker.getTypeAtLocation(base);
  return { ids: symbolIdentities(checker, type.aliasSymbol || type.getSymbol()), origin: valueOrigin, builtin: primitiveName(type) };
}
function compare(ref, actual, location) {
  if (!ref || !ref.Resolution) return;
  if (ref.Resolution === 'undetermined') { undetermined++; reasons[ref.Reason] = (reasons[ref.Reason] || 0) + 1; return; }
  checked++;
  let match = false;
  if (ref.Resolution === 'declared') match = actual.ids.some(id => id.file === ref.File && id.owner === ref.Owner && id.name === ref.Name);
  else if (!ref.Package) {
    match = actual.ids.some(id => id.builtin === ref.Name || (id.file?.startsWith('../') || id.absolute?.includes('/node_modules/')) && id.name === ref.Name && (!ref.Owner || id.owner === ref.Owner));
    if (actual.builtin === ref.Name && !ref.Owner) match = true;
  } else {
    const origin = actual.origin;
    match = !!origin && actual.ids.some(id => id.absolute && outsideRepository(id)) && (!actual.member || actual.ids.every(outsideRepository)) && origin.pkg === ref.Package && (actual.member && !actual.namespace ? ref.Name === actual.member : ref.Name === origin.imported);
    // Types inside containers can originate in the dependency declaration
    // even when the source occurrence is no longer a bare import identifier.
    if (!match && actual.typeNodes) {
      match = actual.typeNodes.some(n => {
        const o = moduleOrigin(actual.checker, n);
        return o?.pkg === ref.Package && o.imported === ref.Name;
      });
    }
  }
  if (match) { agreed++; if (samples.length < 30) samples.push({ location, ref }); }
  else disagreements.push({ location, ref, actual: actual.ids, origin: actual.origin });
}
for (const [rel, facts] of Object.entries(input.Facts).sort()) {
  facts.Calls ||= []; facts.Declarations ||= [];
  if (!facts.ResolvedCallsAvailable && !facts.FieldTypesAvailable) { unavailable++; continue; }
  const absolute = path.join(root, rel), program = programFor(absolute), checker = program.getTypeChecker(), source = program.getSourceFile(absolute);
  if (!source) throw new Error(`compiler did not load ${rel}`);
  const calls = [], fields = [];
  function walk(n) {
    if (ts.isCallExpression(n) || ts.isTaggedTemplateExpression(n)) calls.push(n);
    if (ts.isPropertyDeclaration(n) || ts.isPropertySignature(n) || ts.isParameter(n) && ts.isParameterPropertyDeclaration(n, n.parent)) fields.push(n);
    ts.forEachChild(n, walk);
  }
  walk(source);
  const matched = new Set();
  for (const ref of facts.Calls) {
    if (!facts.ResolvedCallsAvailable) break;
    const index = calls.findIndex((call, i) => {
      if (matched.has(i)) return false;
      const fn = ts.isTaggedTemplateExpression(call) ? call.tag : call.expression;
      const callee = ts.isPropertyAccessExpression(fn) ? fn.name.getText(source) : fn.getText(source).replace(/\s+/g, ' ');
      return source.getLineAndCharacterOfPosition(call.getStart()).line + 1 === ref.Line && callee === ref.Callee;
    });
    const call = calls[index];
    if (call) matched.add(index);
    compare(ref.Target, call ? actualCall(checker, call) : { ids: [] }, `${rel}:${ref.Line}:target`);
    compare(ref.Receiver, call ? receiverActual(checker, call) : { ids: [] }, `${rel}:${ref.Line}:receiver`);
  }
  if (facts.FieldTypesAvailable) for (const field of facts.Declarations.filter(d => d.Kind === 'field')) {
    const n = fields.find(n => n.name?.getText(source) === field.Name && ownerOf(n) === field.Owner && source.getLineAndCharacterOfPosition(n.getStart()).line + 1 === field.StartLine);
    if (!n) throw new Error(`${rel}:${field.StartLine}: compiler has no field ${field.Owner}.${field.Name}`);
    const ids = typeIdentities(checker, n.type), typeNodes = [];
    if (n.questionToken) ids.push({ builtin: primitiveName(checker.getUndefinedType()) });
    function collect(t) { if (!t || ts.isFunctionTypeNode(t) || ts.isConstructorTypeNode(t)) return; if (ts.isTypeReferenceNode(t)) typeNodes.push(t.typeName); ts.forEachChild(t, collect); }
    collect(n.type);
    for (const ref of field.Holds || []) compare(ref, { ids, typeNodes, checker }, `${rel}:${field.StartLine}:field ${field.Owner}.${field.Name}`);
  }
}
const report = { compiler: ts.version, root, checked, agreed, undetermined, unavailable, disagreements, reasons, samples };
console.log(JSON.stringify(report, null, 2));
process.exitCode = disagreements.length ? 1 : 0;
