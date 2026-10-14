// Generate builtin identities and member owners from pinned compiler declarations.
// This is development-time data generation; runtime resolution remains Go-only.
const fs = require('node:fs');
const path = require('node:path');
const compiler = process.env.PATH.split(path.delimiter).map(p => path.join(p, 'tsc')).find(p => {
  try { return fs.realpathSync(p).includes('typescript/bin/tsc'); } catch { return false; }
});
if (!compiler) throw new Error('Run through npm exec --package=typescript@5.9.3 --package=@types/node@24.10.1');
const compilerFile = fs.realpathSync(compiler), ts = require(path.resolve(compilerFile, '../../lib/typescript.js'));
if (ts.version !== '5.9.3') throw new Error(`Expected TypeScript 5.9.3, got ${ts.version}`);
const modules = path.resolve(compilerFile, '../../..');
const nodeTypes = path.join(modules, '@types/node');
const nodePackage = JSON.parse(fs.readFileSync(path.join(nodeTypes, 'package.json'), 'utf8'));
if (nodePackage.version !== '24.10.1') throw new Error(`Expected @types/node 24.10.1, got ${nodePackage.version}`);
const file = path.resolve('/__arclint_builtin_probe__.ts');
const source = 'let primitives: [string, number, boolean, bigint, symbol];';
const options = { target: ts.ScriptTarget.ESNext, lib: ['lib.esnext.d.ts'], types: ['node'], typeRoots: [path.join(modules, '@types')], skipLibCheck: true, noEmit: true };
const host = ts.createCompilerHost(options), getSource = host.getSourceFile.bind(host);
host.getSourceFile = (name, languageVersion, ...rest) => name === file ? ts.createSourceFile(file, source, languageVersion, true) : getSource(name, languageVersion, ...rest);
const program = ts.createProgram([file], options, host), checker = program.getTypeChecker(), probe = program.getSourceFile(file);
const types = new Set(['string','number','boolean','bigint','symbol','null','undefined','void','never','object']);
const values = new Set(), valueTypes = new Map(), members = new Map(), constructors = new Map(), computations = new Map();
function standard(symbol) {
  return (symbol.getDeclarations() || []).some(d => {
    const file = d.getSourceFile().fileName.split(path.sep).join('/');
    return /\/typescript\/lib\/lib\.[^/]+\.d\.ts$/.test(file) || file.startsWith(nodeTypes.split(path.sep).join('/') + '/');
  });
}
function owner(decl) {
  for (let n = decl.parent; n; n = n.parent) {
    if (ts.isClassDeclaration(n) || ts.isInterfaceDeclaration(n) || ts.isTypeAliasDeclaration(n)) return n.name?.text;
    if (ts.isSourceFile(n)) break;
  }
  return ''; // namespace functions have no declaring type
}
function indexMembers(name, type) {
  for (const property of checker.getPropertiesOfType(type)) {
    const owners = new Set((property.getDeclarations() || []).map(owner));
    if (owners.size === 1 && !owners.has(undefined)) members.set(`${name}.${property.getName()}`, [...owners][0]);
  }
}
for (const symbol of checker.getSymbolsInScope(probe, ts.SymbolFlags.Type)) {
  if (!standard(symbol)) continue;
  types.add(symbol.getName());
  for (const declaration of symbol.getDeclarations() || []) {
    if (!ts.isTypeAliasDeclaration(declaration)) continue;
    let reason;
    function computation(node) {
      if (ts.isConditionalTypeNode(node)) reason ||= 'conditional';
      if (ts.isMappedTypeNode(node)) reason ||= 'mapped';
      if (ts.isIndexedAccessTypeNode(node)) reason ||= 'indexed-access';
      ts.forEachChild(node, computation);
    }
    computation(declaration.type);
    if (reason) computations.set(symbol.getName(), `builtin type ${symbol.getName()} requires ${reason} type computation`);
  }
  indexMembers(symbol.getName(), checker.getDeclaredTypeOfSymbol(symbol));
}
for (const symbol of checker.getSymbolsInScope(probe, ts.SymbolFlags.Value)) {
  if (!standard(symbol)) continue;
  const name = symbol.getName(), type = checker.getTypeOfSymbolAtLocation(symbol, probe);
  values.add(name);
  const signatures = type.getConstructSignatures();
  if (signatures.length) {
    const instances = new Set(signatures.map(signature => {
      const result = signature.getReturnType();
      if (result.flags & (ts.TypeFlags.TypeParameter | ts.TypeFlags.Any | ts.TypeFlags.Unknown)) return '';
      return result.aliasSymbol?.getName() || result.getSymbol()?.getName() || '';
    }));
    constructors.set(name, instances.size === 1 ? [...instances][0] : '');
  }
  const typeName = type.aliasSymbol?.getName() || type.getSymbol()?.getName();
  if (typeName && typeName !== '__type') { valueTypes.set(name, typeName); indexMembers(typeName, type); }
}
const tuple = probe.statements[0].declarationList.declarations[0].type;
for (const node of tuple.elements) indexMembers(node.getText(probe), checker.getTypeFromTypeNode(node));
// Global script names can also merge with the compiler's default DOM
// declarations. They are guards only; DOM members are not runtime promises.
const defaultProgram = ts.createProgram([file], { ...options, lib: ['lib.esnext.d.ts','lib.dom.d.ts'] }, host);
const defaultChecker = defaultProgram.getTypeChecker();
const globalNames = new Set([...types,...values]);
for (const symbol of defaultChecker.getSymbolsInScope(defaultProgram.getSourceFile(file), ts.SymbolFlags.Type | ts.SymbolFlags.Value | ts.SymbolFlags.Namespace)) {
 if (standard(symbol)) globalNames.add(symbol.getName());
}
function map(name, entries, valueType) {
  let out = `${name}: map[string]${valueType}{\n`;
  for (const [key, value] of [...entries].sort(([a], [b]) => a.localeCompare(b, 'en'))) out += `\t${JSON.stringify(key)}: ${valueType === 'bool' ? 'true' : JSON.stringify(value)},\n`;
  return out + '},\n';
}
const output = '// Code generated by tools/tsoracle/generate_builtins.cjs; DO NOT EDIT.\n' +
  '// Source: TypeScript 5.9.3 esnext libraries and @types/node 24.10.1.\npackage typescript\n\n' +
  'func builtinDeclarationData() builtinDeclarations { return builtinDeclarations{\n' +
  map('types', [...types].map(n => [n, true]), 'bool') +
  map('values', [...values].map(n => [n, true]), 'bool') +
  map('valueTypes', valueTypes, 'string') +
  map('memberOwners', members, 'string') +
  map('globalNames', [...globalNames].map(n=>[n,true]), 'bool') +
  map('constructors', constructors, 'string') +
  map('computations', computations, 'string') + '}}\n';
fs.writeFileSync(process.argv[2], output);
console.log(`${types.size} builtin types, ${values.size} values, ${members.size} member owners, ${globalNames.size} global merging guards`);
