import { test } from 'node:test';
import assert from 'node:assert/strict';
import { parse, stringify } from 'yaml';
import { importProject } from '../src/serialization';
import { architectureAnchorPaths, architectureImportsForSelection, buildArchitectureEvidence, loadArchitecturePaths, matchesArchitectureSource } from '../src/architecture-evidence';
import type { RepositoryCheckResult, RepositoryContextResult, RepositoryProject, RepositoryDependencies } from '../src/repository';

const domainYaml = `version: 1
project: Architecture evidence
contexts:
  adoption:
    definition: How this repository adopts Rules.
    value_objects:
      Suppression:
        definition: Keeps findings with their gate effect removed.
        invariants:
          names-a-path-with-a-reason: A path and reason are required.
      Override:
        definition: An adopting decision against a Rule.
        invariants:
          names-a-rule: A Rule must be named.
  commerce:
    definition: Handles purchases.
    aggregates:
      Order:
        definition: A purchase.
        identity: OrderID
        assertions:
          lines-frozen:
            on: confirm
            statement: Lines are frozen after confirmation.
    specifications:
      Sellable:
        definition: Whether a product can be sold.
  vocabulary:
    definition: Recorded words without located contract evidence.
`;
function repository(): RepositoryProject {
  return {
    repository: { name: 'real-repository', root: '/repo' }, domainYaml, loadedAt: '2026-09-14T10:00:00Z',
    rulesYaml: `rules:\n  dependencies/layers:\n    layers: [checkout, kernel]\n  dependencies/disabled:\n    layers: [other, kernel]\n`,
    rules: [
      { id: 'dependencies/layers', type: 'layers', severity: 'error', proposition: 'Checkout may import kernel.' },
      { id: 'dependencies/disabled', type: 'layers', severity: 'warning', proposition: 'Disabled order.', disabled: true },
      { id: 'example/distributed/layers', type: 'layers', severity: 'error', proposition: 'Distributed order exists but is not structured in CLI detail.', provenance: 'example/distributed@1.0.0' },
    ],
    context: { Scope: 'repository', Languages: ['go'], RuleCount: 3, Rules: null, Paths: null,
      Zones: [
        { Name: 'checkout', Description: 'Checkout code.', Paths: ['internal/checkout/**'], Internal: ['kernel'], InternalRestricted: true, External: 'forbid', Stdlib: 'allow' },
        { Name: 'kernel', Description: 'Core code.', Paths: ['internal/domain/**'], Internal: null, InternalRestricted: true, External: 'forbid', Stdlib: 'allow' },
        { Name: 'shared', Description: 'Cross-cutting files.', Paths: ['internal/**'], Internal: null, InternalRestricted: false, External: 'allow', Stdlib: 'allow' },
        { Name: 'vocabulary', Description: 'Same spelling is insufficient.', Paths: ['vocab/**'], Internal: null, InternalRestricted: false, External: 'allow', Stdlib: 'allow' },
      ],
      domain: { source: 'domain.arclint.yaml', located: true, contexts: [
        { name: 'adoption', invariants: [
          { owner: 'Suppression', ownerConcept: 'value_object', key: 'names-a-path-with-a-reason', statement: 'A path and reason are required.', source: 'internal/domain/rule/suppression.go:15', anchor: 'found' },
          { owner: 'Override', ownerConcept: 'value_object', key: 'names-a-rule', statement: 'A Rule must be named.', source: 'internal/checkout/override.go:23', anchor: 'found' },
        ] },
        { name: 'commerce', assertions: [{ owner: 'Order', key: 'lines-frozen', statement: 'Lines are frozen after confirmation.', on: 'confirm', source: 'internal/checkout/order.go:40', anchor: 'found' }],
          specifications: [{ name: 'Sellable', source: 'internal/domain/sellable.go:8', anchor: 'found' }] },
        { name: 'vocabulary' },
      ] },
    },
  };
}
function pathResult(path: string, zones: string[], pathType: 'file' | 'directory' | 'missing' = 'file'): RepositoryContextResult {
  return { path, pathType, queriedAt: '2026-09-14T10:01:00Z', report: { Scope: path, Languages: ['go'], RuleCount: 3, Zones: [], Paths: [{ Path: path, Zones: zones }],
    Rules: [{ Summary: { ID: 'dependencies/layers', Type: 'layers', Severity: 'error', Proposition: 'Layers govern Zones.' }, Reason: 'This file belongs to a layered Zone.', Via: null }] } };
}

test('located contracts join every reported Zone without context naming or directory guesses', () => {
  const repo = repository(), model = importProject(domainYaml);
  const evidence = buildArchitectureEvidence(model, repo, [
    pathResult('internal/domain/rule/suppression.go', ['kernel', 'shared']),
    pathResult('internal/checkout/override.go', ['checkout', 'shared']),
    pathResult('internal/checkout/order.go', ['checkout', 'shared']),
    pathResult('internal/domain/sellable.go', ['kernel', 'shared']),
  ]);
  assert.equal(evidence.linked, true);
  const adoption = evidence.contexts.find(context => context.name === 'adoption')!;
  assert.deepEqual(adoption.zones, ['kernel', 'shared', 'checkout']);
  const suppression = adoption.subjects.find(subject => subject.name === 'Suppression')!;
  assert.equal(suppression.ownerId, undefined, 'A constructor anchor does not invent an aggregate owner.');
  assert.deepEqual(suppression.zones, ['kernel', 'shared']);
  assert.equal(suppression.anchors[0].path, 'internal/domain/rule/suppression.go');
  assert.equal(suppression.anchors[0].line, 15);
  assert.equal(suppression.anchors[0].membership, 'reported');
  assert.equal(suppression.anchors[0].rules[0].Summary.ID, 'dependencies/layers');
  const vocabulary = evidence.contexts.find(context => context.name === 'vocabulary')!;
  assert.equal(vocabulary.state, 'unlocated'); assert.deepEqual(vocabulary.zones, []);
  const commerce = evidence.contexts.find(context => context.name === 'commerce')!;
  assert.equal(commerce.subjects.find(subject => subject.name === 'Order')!.anchors[0].kind, 'assertion', 'The CLI assertion record omits ownerConcept.');
  assert.equal(commerce.subjects.find(subject => subject.name === 'Sellable')!.anchors[0].kind, 'specification');
  assert.deepEqual(commerce.zones, ['checkout', 'shared', 'kernel']);
});

test('full semantic source matching ignores mapping order and drawing coordinates but rejects a different domain record', () => {
  const repo = repository(), model = importProject(domainYaml);
  const reordered = parse(domainYaml) as Record<string, unknown>;
  repo.domainYaml = stringify(Object.fromEntries(Object.entries(reordered).reverse()));
  model.contexts[0].position = [100, 200, 300];
  assert.equal(matchesArchitectureSource(model, repo), true);
  model.concepts.find(subject => subject.name === 'Suppression')!.definition = 'A different domain meaning.';
  const evidence = buildArchitectureEvidence(model, repo, [pathResult('internal/domain/rule/suppression.go', ['kernel'])]);
  assert.equal(evidence.linked, false);
  assert.ok(evidence.contexts.every(context => context.state === 'unlinked' && context.zones.length === 0 && context.anchors.length === 0));
  assert.equal(evidence.zones.length, 4, 'Repository architecture can be shown independently of the unrelated model.');
});

test('missing files and failed or absent memberships never become actual Zone links', () => {
  const repo = repository(), model = importProject(domainYaml);
  let evidence = buildArchitectureEvidence(model, repo);
  let suppression = evidence.contexts[0].subjects.find(subject => subject.name === 'Suppression')!;
  assert.equal(suppression.anchors[0].membership, 'unqueried'); assert.deepEqual(suppression.zones, []);
  evidence = buildArchitectureEvidence(model, repo, [pathResult('internal/domain/rule/suppression.go', ['kernel', 'shared'], 'missing'), { path: 'internal/checkout/override.go', error: 'Observation failed.' }]);
  suppression = evidence.contexts[0].subjects.find(subject => subject.name === 'Suppression')!;
  assert.equal(suppression.anchors[0].membership, 'missing'); assert.deepEqual(suppression.zones, []);
  const override = evidence.contexts[0].subjects.find(subject => subject.name === 'Override')!;
  assert.equal(override.anchors[0].membership, 'unavailable'); assert.equal(override.anchors[0].reason, 'Observation failed.');
  const wrong = pathResult('internal/domain/rule/suppression.go', ['kernel']); wrong.report.Paths![0].Path = 'different.go';
  assert.equal(buildArchitectureEvidence(model, repo, [wrong]).contexts[0].subjects[0].anchors[0].membership, 'unavailable');
  repo.context.domain!.located = false;
  assert.equal(buildArchitectureEvidence(model, repo).contexts[0].state, 'unavailable');
});

test('layer order comes only from explicit local Rules and import contracts never stand in for observed edges', () => {
  const evidence = buildArchitectureEvidence(importProject(domainYaml), repository());
  assert.deepEqual(evidence.layers.map(rule => ({ id: rule.id, zones: rule.zones, disabled: rule.disabled })), [
    { id: 'dependencies/layers', zones: ['checkout', 'kernel'], disabled: false },
    { id: 'dependencies/disabled', zones: ['other', 'kernel'], disabled: true },
  ]);
  assert.deepEqual(evidence.unavailableLayerRuleIds, ['example/distributed/layers']);
  assert.equal(evidence.observedImports.state, 'unavailable');
  assert.deepEqual(evidence.zones[0].internal, ['kernel']);
  assert.equal(evidence.zones[1].internalRestricted, true); assert.deepEqual(evidence.zones[1].internal, []);
  assert.equal(evidence.zones[2].internalRestricted, false);
});

test('reported occurrences keep individual Baseline statuses while missing checks and outcomes remain unknown', () => {
  const repo = repository(), model = importProject(domainYaml), path = 'internal/domain/rule/suppression.go';
  repo.context.domain!.contexts![0].invariants![1].source = `${path}:25`;
  const finding = { kind: 'violation', ruleId: 'some/rule', path, message: 'Same evidence', status: 'baselined' };
  const check: RepositoryCheckResult = { exitCode: 0, checkedAt: '2026-09-14T10:02:00Z', stderr: '', outcomesAvailable: false,
    diagnostics: [{ ...finding, line: 15 }, { ...finding, line: 20 }, { ...finding, line: 30, status: 'active' }, { kind: 'coverage', message: 'Rule not evaluated' }] };
  const evidence = buildArchitectureEvidence(model, repo, [pathResult(path, ['kernel'])], check);
  assert.equal(evidence.evaluation.counts.baselined, 2); assert.equal(evidence.evaluation.counts.active, 1);
  assert.equal(evidence.evaluation.counts.coverage, 1); assert.equal(evidence.evaluation.outcomesAvailable, false);
  const subject = evidence.contexts[0].subjects[0]; assert.equal(subject.diagnostics.length, 3);
  assert.equal(evidence.contexts[0].subjects[1].diagnostics.length, 3);
  assert.equal(evidence.contexts[0].diagnostics.length, 3, 'Sharing a file never doubles the context occurrence count.');
  assert.equal('status' in subject, false, 'A term or Rule does not inherit a finding-level Baseline status.');
  const unrun = buildArchitectureEvidence(model, repo);
  assert.equal(unrun.evaluation.state, 'not-run'); assert.equal(unrun.evaluation.outcomesAvailable, false);
  const empty = buildArchitectureEvidence(model, repo, [], { ...check, diagnostics: [] });
  assert.equal(empty.evaluation.state, 'reported'); assert.equal(empty.evaluation.outcomesAvailable, false);
});

test('membership loading uses distinct real CLI anchors, bounded requests, and visible partial failure', async () => {
  const repo = repository();
  repo.context.domain!.contexts![0].invariants!.push(
    { owner: 'Suppression', ownerConcept: 'value_object', key: 'second', statement: 'Another key.', source: 'internal/domain/rule/suppression.go:16', anchor: 'found' },
    { key: 'not-located', statement: 'Missing.', source: 'invented.go:1', anchor: 'missing' },
    { key: 'unsafe', statement: 'Unsafe.', source: '../outside.go:1', anchor: 'found' },
  );
  const paths = architectureAnchorPaths(repo); assert.equal(paths.length, 4); assert.equal(paths.includes('invented.go'), false);
  let current = 0, peak = 0;
  const calls: string[] = [];
  const results = await loadArchitecturePaths(repo, undefined, async path => {
    calls.push(path); current++; peak = Math.max(peak, current);
    await new Promise(resolve => setTimeout(resolve, 5)); current--;
    if (path.endsWith('override.go')) throw new Error('Could not observe this path.');
    return pathResult(path, ['shared']);
  });
  assert.equal(peak, 2); assert.deepEqual(calls.sort(), paths);
  assert.equal(results.length, 4); assert.equal(results.filter(result => 'error' in result).length, 1);
});

test('the selected source is queried first and completed memberships appear before the slowest path', async () => {
  const repo = repository(), paths = architectureAnchorPaths(repo), selected = paths.at(-1)!;
  let release!: () => void;
  const delayed = new Promise<void>(resolve => release = resolve);
  const calls: string[] = [], progress: string[][] = [];
  const loading = loadArchitecturePaths(repo, undefined, async path => {
    calls.push(path);
    if (path !== selected) await delayed;
    return pathResult(path,['shared']);
  }, {priority:() => [selected],progress:partial => progress.push(partial.map(result => result.path))});
  await new Promise(resolve => setTimeout(resolve,10));
  assert.equal(calls[0],selected);
  assert.deepEqual(progress,[[selected]],'The selected membership does not wait for unrelated source queries.');
  release();
  assert.deepEqual((await loading).map(result => result.path), paths, 'Completion order does not reorder the result.');
});

function dependencies(): RepositoryDependencies {
  return {
    files: [
      { path: 'internal/domain/rule/suppression.go', zones: ['kernel', 'shared'], language: 'go', importsAvailable: true },
      { path: 'internal/checkout/order.go', zones: ['checkout', 'shared'], language: 'go', importsAvailable: true },
      { path: 'internal/domain/readme.md', zones: ['kernel', 'shared'], language: '', importsAvailable: false },
    ],
    edges: [
      { sourcePath: 'internal/checkout/order.go', targetPath: 'internal/domain/rule', targetKind: 'directory', specifier: 'example.com/internal/domain/rule', line: 3, classification: 'internal', sourceZones: ['checkout', 'shared'], targetZones: ['kernel', 'shared'] },
      { sourcePath: 'internal/checkout/order.go', targetPath: 'internal/domain/rule/suppression.go', targetKind: 'file', specifier: './suppression', line: 4, classification: 'internal', sourceZones: ['checkout', 'shared'], targetZones: ['kernel', 'shared'] },
      { sourcePath: 'internal/domain/rule/suppression.go', targetPath: 'internal/checkout', targetKind: 'directory', specifier: 'example.com/internal/checkout', line: 6, classification: 'internal', sourceZones: ['kernel', 'shared'], targetZones: ['checkout', 'shared'] },
      { sourcePath: 'internal/domain/rule/suppression.go', targetPath: '', targetKind: 'unresolved', specifier: 'fmt', line: 7, classification: 'stdlib', sourceZones: ['kernel', 'shared'], targetZones: [] },
    ],
    coverage: { scope: 'repository', languages: ['go'], filesObserved: 3, sourceFiles: 2, filesWithImports: 2, complete: true },
    diagnostics: [], limitations: ['Runtime dependencies are not observed.'], observedAt: '2026-09-15T15:00:00Z', revision: 'observed-fixture', changedDuringObservation: false,
  };
}

test('observed file memberships populate every overlapping Zone without inventing directory target files', () => {
  const evidence = buildArchitectureEvidence(importProject(domainYaml), repository(), [], null, dependencies());
  assert.equal(evidence.observedImports.state, 'reported');
  assert.deepEqual(evidence.zones.find(zone => zone.name === 'kernel')!.observedPaths, ['internal/domain/readme.md', 'internal/domain/rule/suppression.go']);
  assert.equal(evidence.zones.find(zone => zone.name === 'shared')!.observedPaths!.length, 3);
  assert.deepEqual(evidence.zones.find(zone => zone.name === 'vocabulary')!.observedPaths, [], 'Same-named domain context never adds files to a Zone.');
  assert.equal(evidence.observedImports.edges[0].targetKind, 'directory');
  assert.equal(evidence.observedImports.edges[0].targetPath, 'internal/domain/rule');
  assert.deepEqual(evidence.observedImports.edges[0].targetZones, ['kernel', 'shared']);
  assert.equal(architectureImportsForSelection(evidence, null, null).edges.length, 3, 'Only actual internal imports with local targets enter the repository graph.');
});

test('selection dependencies use exact located files, preserve repeated imports, and do not expand package targets into domain ownership', () => {
  const model = importProject(domainYaml), observed = dependencies();
  observed.edges.push({ ...observed.edges[1], line: 8 });
  const evidence = buildArchitectureEvidence(model, repository(), [pathResult('internal/domain/rule/suppression.go', ['kernel', 'shared'])], null, observed);
  const subject = model.concepts.find(item => item.name === 'Suppression')!;
  const scoped = architectureImportsForSelection(evidence, subject.id, subject.contextId);
  assert.equal(scoped.scope, 'located-files');
  assert.deepEqual(scoped.anchorPaths, ['internal/domain/rule/suppression.go']);
  assert.deepEqual(scoped.edges.map(edge => edge.line), [4, 6, 8], 'An import to the containing Go package is not an import to this specific anchored file.');
  const unlocated = model.concepts.find(item => item.name === 'Override')!;
  assert.deepEqual(architectureImportsForSelection(evidence, unlocated.id, unlocated.contextId).edges, []);
  assert.deepEqual(architectureImportsForSelection(evidence, 'unknown-selection', subject.contextId).edges, []);
});

test('repository observations remain separate from unlinked domain drafts and unavailable data never means an empty observation', () => {
  const model = importProject(domainYaml), repo = repository();
  const missing = buildArchitectureEvidence(model, repo);
  assert.equal(missing.zones[0].observedPaths, null);
  const loading = buildArchitectureEvidence(model, repo, [], null, { state: 'loading', reason: 'Observing imports.' });
  assert.equal(loading.observedImports.state, 'loading');
  assert.equal(loading.zones[0].observedPaths, null);
  const observed = dependencies(); observed.coverage.complete = false;
  observed.diagnostics.push({ code: 'scan-failed', path: 'unreadable.go', message: 'Cannot inspect this file.' });
  model.concepts[0].definition = 'An unsaved draft meaning.';
  const evidence = buildArchitectureEvidence(model, repo, [pathResult('internal/domain/rule/suppression.go', ['kernel'])], null, observed);
  assert.equal(evidence.linked, false);
  assert.equal(evidence.observedImports.state, 'reported');
  assert.match(evidence.observedImports.reason, /incomplete/);
  assert.equal(architectureImportsForSelection(evidence, null, null).edges.length, 3);
  assert.equal(architectureImportsForSelection(evidence, model.concepts[0].id, model.contexts[0].id).edges.length, 0);
});

test('native file observation resolves pending anchor memberships without fabricating applicable Rules', () => {
  const model = importProject(domainYaml), repo = repository(), observed = dependencies();
  const subject = model.concepts.find(item => item.name === 'Suppression')!;
  const evidence = buildArchitectureEvidence(model, repo, [], null, observed);
  const anchor = evidence.contexts[0].subjects.find(item => item.id === subject.id)!.anchors[0];
  assert.equal(anchor.membership, 'reported');
  assert.deepEqual(anchor.zones, ['kernel', 'shared']);
  assert.deepEqual(anchor.rules, [], 'Native membership does not supply the path-specific applicable Rule list.');
  assert.match(anchor.reason!, /Rules are still being queried/);
  const scoped = architectureImportsForSelection(evidence, subject.id, subject.contextId);
  assert.equal(scoped.resolution, 'known');
  assert.deepEqual(scoped.edges.map(edge => edge.line), [4, 6], 'Imports can appear before the slower path context command finishes.');

  const late = buildArchitectureEvidence(model, repo, [pathResult(anchor.path, ['kernel'])], null, observed);
  const resolved = late.contexts[0].subjects.find(item => item.id === subject.id)!.anchors[0];
  assert.deepEqual(resolved.zones, ['kernel'], 'The later exact-path result takes precedence over the earlier repository observation.');
  assert.equal(resolved.rules[0].Summary.ID, 'dependencies/layers');
  const removed = buildArchitectureEvidence(model, repo, [pathResult(anchor.path, [], 'missing')], null, observed);
  assert.equal(architectureImportsForSelection(removed, subject.id, subject.contextId).resolution, 'unavailable');
  assert.deepEqual(architectureImportsForSelection(removed, subject.id, subject.contextId).edges, []);
});

test('an observed import list does not turn unresolved source selection into a known empty result', () => {
  const model = importProject(domainYaml), repo = repository(), observed = dependencies();
  const subject = model.concepts.find(item => item.name === 'Suppression')!;
  observed.files = []; observed.edges = [];
  const pending = architectureImportsForSelection(buildArchitectureEvidence(model, repo, [], null, observed), subject.id, subject.contextId);
  assert.deepEqual(pending.edges, []);
  assert.equal(pending.resolution, 'pending');
  assert.match(pending.reason, /Resolving source/);
  observed.files.push({ path: 'internal/domain/rule/suppression.go', zones: [], language: 'go', importsAvailable: true });
  const known = architectureImportsForSelection(buildArchitectureEvidence(model, repo, [], null, observed), subject.id, subject.contextId);
  assert.equal(known.resolution, 'known', 'An observed unzoned file still establishes the exact source selection.');
  assert.deepEqual(known.edges, []);
  assert.equal(known.anchorPaths.length, 1);
});
