// Black-box checks that the oracle rejects incorrect references. Run against
// real adapter output with declared calls, external calls, receivers and fields.
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const input = JSON.parse(fs.readFileSync(process.argv[2], 'utf8'));
const scratch = fs.mkdtempSync(path.join(os.tmpdir(), 'arclint-oracle-negative-'));
const mutations = [
  ['declared target', i => Object.values(i.Facts).flatMap(f => f.Calls || []).map(c => c.Target).find(r => r.Resolution === 'declared'), r => { r.Name = '__incorrect_declaration__'; }],
  ['external target', i => Object.values(i.Facts).flatMap(f => f.Calls || []).map(c => c.Target).find(r => r.Resolution === 'external' && r.Package), r => { r.Package = '__incorrect_module__'; }],
  ['builtin owner', i => Object.values(i.Facts).flatMap(f => f.Calls || []).map(c => c.Target).find(r => r.Resolution === 'external' && !r.Package && r.Owner), r => { r.Owner = '__incorrect_owner__'; }],
  ['declared receiver', i => Object.values(i.Facts).flatMap(f => f.Calls || []).map(c => c.Receiver).find(r => r.Resolution === 'declared'), r => { r.Name = '__incorrect_type__'; }],
  ['field type', i => Object.values(i.Facts).flatMap(f => f.Declarations || []).flatMap(d => d.Holds || []).find(r => r.Resolution === 'declared'), r => { r.Name = '__incorrect_field_type__'; }],
];
try {
  for (const [name, select, mutate] of mutations) {
    const copy = structuredClone(input), ref = select(copy);
    if (!ref) throw new Error(`Input has no ${name} for negative control`);
    mutate(ref);
    const file = path.join(scratch, 'facts.json');
    fs.writeFileSync(file, JSON.stringify(copy));
    const result = spawnSync(process.execPath, [path.join(__dirname, 'check.cjs'), file], { encoding: 'utf8', maxBuffer: 32 * 1024 * 1024 });
    if (result.error) throw result.error;
    if (result.status !== 1) throw new Error(`${name}: expected rejection, status ${result.status}, stderr ${result.stderr}`);
    const report = JSON.parse(result.stdout);
    if (!report.disagreements.length) throw new Error(`${name}: failed without a disagreement`);
    console.log(`${name}: rejected (${report.disagreements.length} disagreement)`);
  }
} finally {
  fs.rmSync(scratch, { recursive: true, force: true });
}
