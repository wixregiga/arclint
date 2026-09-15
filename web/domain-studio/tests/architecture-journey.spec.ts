import { expect, test, type Locator, type Page } from '@playwright/test';
import { importProject, exportDomainYaml } from '../src/serialization';
import type { RepositoryBaselinePreview, RepositoryCheckResult, RepositoryContextReport, RepositoryDependencies, RepositoryProject } from '../src/repository';
import { closeWorkSurface, openEditor, savedProject, selectPlace } from './studio.helpers';

// Deliberately unrelated to ArcLint's own domain. Contexts and Zones overlap rather
// than sharing a naming convention or a one-to-one hierarchy.
const domainYaml = `version: 1
project: Dispatch desk
contexts:
  fulfilment:
    definition: Preparing a customer's shipment.
    aggregates:
      Shipment:
        definition: A dispatch with its own identity and readiness promises.
        identity: ShipmentID
        invariants:
          address-known: Every shipment has a destination address.
          items-tracked: Every shipment item has a tracking reference.
        assertions:
          dispatch-ready:
            on: Dispatch
            statement: Every item is ready before dispatch.
    value_objects:
      ShipmentID:
        definition: The stable identity of a dispatch.
  invoicing:
    definition: Recording charges for completed work.
    value_objects:
      Money:
        definition: An amount expressed in one currency.
        invariants:
          currency-known: Every amount has a currency.
    questions:
      unsettled-reference: What identifies a disputed invoice?
`;
const model = importProject(domainYaml);
const shipment = model.concepts.find(concept => concept.name === 'Shipment')!;
const money = model.concepts.find(concept => concept.name === 'Money')!;
const fulfilment = model.contexts.find(context => context.name === 'fulfilment')!;
const layerRule = { id: 'dispatch/inward', type: 'layers', severity: 'error', proposition: 'checkout imports only its own layer or kernel.', rationale: 'The dispatch interface depends on the stable kernel.', assurance: 'exact' };
const zones = [
  { Name: 'checkout', Description: 'Dispatch entry points.', Paths: ['app/**'], Internal: ['kernel'], InternalRestricted: true, External: 'forbid', Stdlib: 'allow' },
  { Name: 'kernel', Description: 'Core shipment and amount code.', Paths: ['core/**'], Internal: [], InternalRestricted: true, External: 'forbid', Stdlib: 'allow' },
  { Name: 'audited', Description: 'Code whose promises are reviewed.', Paths: ['app/**', 'core/**'], Internal: null, InternalRestricted: false, External: 'allow', Stdlib: 'allow' },
];
const pathZones: Record<string, string[]> = {
  'app/shipment.go': ['checkout', 'audited'],
  'core/shipment_rules.go': ['kernel', 'audited'],
  'core/money.go': ['kernel', 'audited'],
};
const contextReport: RepositoryContextReport = {
  Scope: 'repository', Languages: ['go'], RuleCount: 1, Zones: zones, Rules: null, Paths: null,
  domain: {
    source: 'domain.arclint.yaml', located: true,
    contexts: [
      { name: 'fulfilment', aggregates: [{ name: 'Shipment', identity: 'ShipmentID' }], invariants: [
        { key: 'address-known', owner: 'Shipment', ownerConcept: 'aggregate', statement: 'Every shipment has a destination address.', source: 'app/shipment.go:12', anchor: 'found' },
        { key: 'items-tracked', owner: 'Shipment', ownerConcept: 'aggregate', statement: 'Every shipment item has a tracking reference.', source: 'core/shipment_rules.go:18', anchor: 'found' },
      ], assertions: [{ key: 'dispatch-ready', owner: 'Shipment', on: 'Dispatch', statement: 'Every item is ready before dispatch.', source: 'app/shipment.go:32', anchor: 'found' }] },
      { name: 'invoicing', valueObjects: ['Money'], invariants: [{ key: 'currency-known', owner: 'Money', ownerConcept: 'value_object', statement: 'Every amount has a currency.', source: 'core/money.go:8', anchor: 'found' }] },
    ],
  },
};
const repository: RepositoryProject = {
  repository: { name: 'dispatch-desk', root: '/workspace/dispatch-desk' }, domainYaml,
  rulesYaml: `version: 1\nrules:\n  dispatch/inward:\n    layers: [checkout, kernel]\n    rationale: The dispatch interface depends on the stable kernel.\n`,
  context: contextReport, rules: [layerRule], loadedAt: '2026-09-14T12:00:00Z',
};
const emptyReport: RepositoryCheckResult = { exitCode: 0, diagnostics: [], checkedAt: '2026-09-14T12:01:00Z', stderr: '', outcomesAvailable: false };

interface FixtureOptions {
  dependencies?: RepositoryDependencies;
  beforeContext?: (path: string) => Promise<void>;
  beforeRule?: () => Promise<void>;
  beforeCheck?: (request: number) => Promise<void>;
  reports?: RepositoryCheckResult[];
  baselinePreview?: RepositoryBaselinePreview;
}
async function openFixture(page: Page, options: FixtureOptions = {}) {
  let checks = 0, revision = 1;
  await page.route('**/api/arclint/**', async route => {
    const url = new URL(route.request().url());
    const endpoint = url.pathname.split('/').at(-1);
    let response: unknown;
    if (endpoint === 'project') response = repository;
    else if (endpoint === 'revision') response = { revision: String(revision), changedAt: '2026-09-14T12:00:00Z', watching: true };
    else if (endpoint === 'dependencies') {
      if (!options.dependencies) { await route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ error: { code: 'DEPENDENCIES_UNAVAILABLE', message: 'Observed imports unavailable for this repository.' } }) }); return; }
      response = options.dependencies;
    }
    else if (endpoint === 'context') {
      const path = url.searchParams.get('path');
      const zone = url.searchParams.get('zone');
      if (path) await options.beforeContext?.(path);
      const memberships = path ? pathZones[path] ?? [] : zone ? [zone] : [];
      response = {
        path: path ?? zone ?? '.', ...(zone ? { zone } : {}), pathType: path && !pathZones[path] ? 'missing' : 'file', queriedAt: '2026-09-14T12:00:01Z',
        report: {
          ...contextReport, Scope: path ?? zone ?? 'repository', Zones: zones.filter(item => memberships.includes(item.Name)),
          Paths: path ? [{ Path: path, Zones: memberships }] : Object.entries(pathZones).filter(([, members]) => members.includes(zone!)).map(([Path, Zones]) => ({ Path, Zones })),
          Rules: [{ Summary: { ID: layerRule.id, Type: layerRule.type, Severity: layerRule.severity, Proposition: layerRule.proposition, Rationale: layerRule.rationale, Assurance: 'exact' }, Reason: 'The selected path belongs to a configured layer.', Via: memberships }],
        },
      };
    } else if (endpoint === 'rule') {
      await options.beforeRule?.();
      response = { summary: layerRule, zones: ['checkout', 'kernel'], evidence: 'Configured layer order compared with imports.', limitations: ['This fixture supplies no observed import graph.'] };
    }
    else if (endpoint === 'check') {
      const request = checks++;
      await options.beforeCheck?.(request);
      response = options.reports?.[Math.min(request, options.reports.length - 1)] ?? emptyReport;
    }
    else if (url.pathname.endsWith('/baseline/preview') && options.baselinePreview) response = options.baselinePreview;
    else if (endpoint === 'files') response = { directory: '.', entries: [{ path: 'app', name: 'app', kind: 'directory' }], truncated: false, queriedAt: '2026-09-14T12:00:01Z' };
    else if (endpoint === 'patterns') response = { patterns: [], queriedAt: '2026-09-14T12:00:01Z' };
    else { await route.fulfill({ status: 404, contentType: 'application/json', body: JSON.stringify({ error: { code: 'UNEXPECTED_TEST_ROUTE', message: `No fixture for ${endpoint}` } }) }); return; }
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(response) });
  });
  await page.goto('/');
  await expect(page.locator('#project-name')).toHaveText('Dispatch desk');
  return { changeRevision: () => { revision++; }, checkCount: () => checks };
}

async function tabActivate(page: Page, target: Locator) {
  await expect(target).toBeVisible();
  for (let index = 0; index < 100; index++) {
    if (await target.evaluate(element => element === document.activeElement)) {
      await page.keyboard.press('Enter');
      return;
    }
    await page.keyboard.press('Tab');
  }
  throw new Error(`Control is not reachable with Tab: ${await target.getAttribute('id') ?? await target.getAttribute('aria-label')}`);
}

async function openDetails(page: Page, section: 'Protection' | 'Inspection' | 'Files & Zones' = 'Files & Zones') {
  const reading = page.locator('#architecture-reading');
  if (!await reading.isVisible()) await page.locator('#architecture-open-reading').click();
  await expect(reading).toBeVisible();
  const disclosure = reading.locator('details').filter({ has: page.locator('summary').filter({ hasText: new RegExp(`^${section}`) }) });
  if (!await disclosure.evaluate(element => (element as HTMLDetailsElement).open)) await disclosure.locator('summary').click();
  await expect(disclosure).toHaveAttribute('open', '');
  return disclosure;
}

async function showZones(page: Page, shown = true) {
  const control = page.locator('#zone-overlay');
  if (await control.getAttribute('aria-pressed') !== String(shown)) await control.click();
  await expect(control).toHaveAttribute('aria-pressed', String(shown));
}

function checkGate() {
  let release!: () => void, requested!: () => void;
  const pending = new Promise<void>(resolve => { release = resolve; });
  const started = new Promise<void>(resolve => { requested = resolve; });
  return { release, started, wait: async () => { requested(); await pending; } };
}

async function closeDetails(page: Page) {
  if (await page.locator('#architecture-reading').isVisible()) await page.locator('#architecture-reading .architecture-reading-toggle').click();
  await expect(page.locator('#architecture-reading')).not.toBeVisible();
}

test('opening a meaning reveals its exact promises and operation contracts in place', async ({ page }, testInfo) => {
  await openFixture(page);
  await selectPlace(page, /^Shipment$/);
  const before = exportDomainYaml(await savedProject(page));
  await expect(page.locator('#architecture-reading')).not.toBeVisible();
  await expect(page.locator('#lens-meaning, #lens-structure, #lens-inspection, #view-prev, #view-next, #view-page')).toHaveCount(0);
  await expect(page.locator('#scene').getByText(/No located anchors/i)).toHaveCount(0);
  await expect(page.locator('#scene [data-inspect-path], #scene [data-inspect-zone]')).toHaveCount(0);
  expect(await page.getByText(shipment.definition, { exact: true }).evaluateAll(elements => elements.filter(element => element.getClientRects().length && getComputedStyle(element).visibility !== 'hidden').length)).toBe(1);
  const invariant = page.locator('#scene [data-inspect-contract="address-known"][data-contract-kind="invariant"]');
  await expect(invariant).toBeVisible();
  await expect(invariant).toHaveAccessibleName(/address-known.*Every shipment has a destination address/);
  await invariant.click();
  const reading = page.locator('#architecture-reading');
  await expect(reading).toContainText('address-known');
  await expect(reading).toContainText('Every shipment has a destination address.');
  const assertion = page.locator('#scene [data-inspect-contract="dispatch-ready"][data-contract-kind="assertion"]');
  await assertion.click();
  await expect(reading).toContainText('dispatch-ready');
  await expect(reading).toContainText('Dispatch');
  await expect(reading).toContainText('Every item is ready before dispatch.');
  await expect(page.locator('#inspector')).not.toBeVisible();
  await expect(page.locator('#scene')).toHaveAttribute('data-selected-id', shipment.id);
  expect(exportDomainYaml(await savedProject(page))).toEqual(before);
  await closeDetails(page);
  await expect(page.locator('.study-scene')).toHaveAttribute('data-camera-state', 'settled');
  await page.screenshot({ path: testInfo.outputPath('opened-shipment-contract.png'), fullPage: true });
  await page.locator('#ascend').click();
  await expect(page.locator('#scene')).toHaveAttribute('data-scope', fulfilment.id);
  await expect(page.locator('#scene')).toHaveAttribute('data-depth', 'context');
});

test('Layers preserve protections and expose overlapping memberships without inventing observed dependencies', async ({ page }, testInfo) => {
  await openFixture(page);
  await selectPlace(page, /^Shipment$/);
  const before = await savedProject(page);
  await showZones(page);
  const reading = page.locator('#architecture-reading');
  await expect(reading).not.toBeVisible();
  await expect(page.locator('#scene [data-inspect-contract="address-known"]')).toBeVisible();
  await expect(page.locator('#scene [data-inspect-contract="dispatch-ready"]')).toBeVisible();
  await expect(page.locator('#scene [data-inspect-path="app/shipment.go"]')).toBeVisible();
  await openDetails(page);
  await expect(reading).toContainText('app/shipment.go');
  await expect(reading).toContainText('core/shipment_rules.go');
  for (const name of ['checkout', 'kernel', 'audited']) await expect(reading).toContainText(name);
  await expect(reading).toContainText(/observed imports.*(?:not supplied|unavailable|unknown)|(?:no|without).*observed import/i);
  await closeDetails(page);
  await page.locator('#ascend').click();
  await expect(page.locator('#scene')).toHaveAttribute('data-scope', fulfilment.id);
  await expect(page.locator('#scene')).toHaveAttribute('data-depth', 'context');
  const key = page.locator('.scene-zone-key [data-zone-name][data-zone-color]');
  await expect(key).toHaveCount(2);
  const colors = await key.evaluateAll(entries => entries.map(entry => entry.getAttribute('data-zone-color')));
  expect(new Set(colors).size, 'The two declared layers have distinct keys; audited remains a membership, not an invented layer').toBe(2);
  await selectPlace(page, /^Shipment$/);
  await expect(page.locator('#architecture-reading')).not.toBeVisible();
  await expect(page.locator('#scene')).toHaveAttribute('data-selected-id', shipment.id);
  await page.locator('#architecture-layer').selectOption(layerRule.id);
  await expect(page.locator('#architecture-layer')).toHaveValue(layerRule.id);
  await closeDetails(page);
  await page.locator('#scene [data-inspect-zone="checkout"]').first().click();
  await expect(page.locator('#evidence-card')).toContainText('Zone: checkout');
  await page.locator('#close-evidence').click();
  await expect(page.locator('#scene')).toHaveAttribute('data-selected-id', shipment.id);
  await page.locator('#governing-rules').click();
  await expect(page.locator('#evidence-card')).toContainText('Located code for Shipment');
  await page.locator('#evidence-card [data-inspect-path="app/shipment.go"]').first().click();
  await page.locator('#evidence-card [data-rule-detail="dispatch/inward"]').first().click();
  await expect(page.locator('#evidence-card')).toContainText(layerRule.rationale);
  await expect(page.locator('#evidence-card')).toContainText('Analysis limits');
  await page.locator('#close-evidence').click();
  await page.locator('#representation-table').click();
  await expect(page.locator('#scene')).toHaveAttribute('data-selected-id', shipment.id);
  await page.locator('#representation-spatial').click();
  expect(await savedProject(page)).toEqual(before);
  await expect(page.locator('#architecture-reading')).not.toBeVisible();
  await expect(page.locator('.study-scene')).toHaveAttribute('data-camera-state', 'settled');
  await page.screenshot({ path: testInfo.outputPath('shipment-zone-structure.png'), fullPage: true });
});

test('independent layers, directed imports and description persist through navigation and reload', async ({ page }, testInfo) => {
  const dependencies: RepositoryDependencies = {
    files: Object.entries(pathZones).map(([path, zones]) => ({ path, zones, language: 'go', importsAvailable: true })),
    edges: [{ sourcePath: 'app/shipment.go', targetPath: 'core', targetKind: 'directory', specifier: 'dispatch/core', line: 4, classification: 'internal', sourceZones: ['checkout', 'audited'], targetZones: ['kernel', 'audited'] }],
    coverage: { scope: 'repository', languages: ['go'], filesObserved: 3, sourceFiles: 3, filesWithImports: 3, complete: true },
    diagnostics: [], limitations: ['Package resolution does not identify an exact target file.'], observedAt: '2026-09-14T12:00:01Z', revision: '1', changedDuringObservation: false,
  };
  await openFixture(page, { dependencies });
  await selectPlace(page, /^Shipment$/);
  const original = exportDomainYaml(await savedProject(page));
  const scene = page.locator('.study-scene');
  const route = page.locator('#scene [data-dependency-source="app/shipment.go"][data-dependency-target="core"]');
  await expect(route).toHaveAttribute('data-dependency-target-kind', 'directory');
  await expect(scene).toHaveAttribute('data-dependency-count', '1');
  await showZones(page);
  await expect(page.locator('#architecture-layer')).toHaveValue(layerRule.id);
  await expect(page.locator('#scene [data-layer-zone="checkout"]')).toHaveCount(1);
  await page.locator('#architecture-layer-options summary').click();
  await page.locator('#architecture-layer-zones').getByLabel('checkout').uncheck();
  await expect(page.locator('#scene [data-layer-zone="checkout"]')).toHaveCount(0);
  await expect(route).toBeVisible();
  await page.locator('#architecture-layer-spread').fill('0.2');
  await page.locator('#architecture-layer-options summary').click();
  await page.locator('#dependency-overlay').click();
  await expect(scene).toHaveAttribute('data-dependency-count', '0');
  await expect(scene).toHaveAttribute('data-layers-visible', 'true');
  await page.locator('#description-overlay').click();
  await expect(page.locator('#place-summary')).not.toBeVisible();
  await page.locator('#representation-table').click();
  await page.locator('#representation-spatial').click();
  await page.locator('#ascend').click();
  await expect(page.locator('#place-name')).toHaveText('fulfilment');
  await selectPlace(page, /^Shipment$/);
  await page.reload();
  await expect(page.locator('#place-name')).toHaveText('Shipment');
  await expect(page.locator('#zone-overlay')).toHaveAttribute('aria-pressed', 'true');
  await expect(page.locator('#dependency-overlay')).toHaveAttribute('aria-pressed', 'false');
  await expect(page.locator('#description-overlay')).toHaveAttribute('aria-pressed', 'false');
  await expect(page.locator('#architecture-layer-spread')).toHaveValue('0.2');
  expect(exportDomainYaml(await savedProject(page))).toEqual(original);
  await page.locator('#dependency-overlay').click();
  await expect(route).toBeVisible();
  await expect(scene).toHaveAttribute('data-camera-state', 'settled');
  await page.screenshot({ path: testInfo.outputPath('layers-and-observed-import.png'), fullPage: true });
});

test('an unrelated imported model with identical names never inherits repository source mappings', async ({ page }) => {
  await openFixture(page);
  await selectPlace(page, /^Shipment$/);
  await showZones(page);
  await openDetails(page);
  await expect(page.locator('#architecture-reading')).toContainText('app/shipment.go');
  await closeDetails(page);
  await selectPlace(page, /^ShipmentID$/);
  await openDetails(page);
  await expect(page.locator('#architecture-reading')).toContainText(/no.*(?:located|source)|unlocated|unknown/i);
  await expect(page.locator('#scene [data-inspect-path="app/shipment.go"]')).toHaveCount(0);
  const foreignYaml = domainYaml.replace('A dispatch with its own identity and readiness promises.', 'A separate museum collection object that happens to share this name.');
  await page.locator('#import-file').setInputFiles({ name: 'unrelated-domain.yaml', mimeType: 'application/yaml', buffer: Buffer.from(foreignYaml) });
  await selectPlace(page, /^Shipment$/);
  await showZones(page);
  await openDetails(page);
  const reading = page.locator('#architecture-reading');
  await expect(reading).toContainText(/unlinked|not linked|does not match|different.*model|model differs|source.*unavailable/i);
  await expect(page.locator('#scene [data-inspect-path="app/shipment.go"]')).toHaveCount(0);
  await expect(page.locator('#scene')).toHaveAttribute('data-selected-id', shipment.id);
  expect(exportDomainYaml(await savedProject(page))).toContain('A separate museum collection object');
});

test('selecting a Layer Rule for an unlocated entry explains the gap without drawing unrelated Zone platforms', async ({ page }) => {
  await openFixture(page);
  const unresolved = model.concepts.find(concept => concept.name === 'unsettled-reference')!;
  await selectPlace(page, /^unsettled-reference/);
  await showZones(page);
  const before = await savedProject(page);
  await page.locator('#architecture-layer').selectOption(layerRule.id);
  await expect(page.locator('#architecture-layer')).toHaveValue(layerRule.id);
  await openDetails(page);
  await expect(page.locator('#architecture-reading')).toContainText('No source association is available for this selection.');
  await expect(page.locator('#architecture-reading')).toContainText('checkout → kernel');
  await closeDetails(page);
  await expect(page.locator('#scene [data-inspect-rule="dispatch/inward"]')).toHaveCount(0);
  await expect(page.locator('#scene [data-inspect-zone], #scene [data-inspect-path]')).toHaveCount(0);
  await expect(page.locator('#scene')).toHaveAttribute('data-selected-id', unresolved.id);
  await expect(page.locator('#scene')).toHaveAttribute('data-scope', unresolved.contextId);
  await expect(page.locator('#place-name')).toHaveText('unsettled-reference');
  await page.locator('#architecture-read-layer').click();
  await expect(page.locator('#evidence-card')).toContainText(layerRule.rationale);
  await page.locator('#close-evidence').click();
  await expect(page.locator('#scene')).toHaveAttribute('data-selected-id', unresolved.id);
  expect(await savedProject(page)).toEqual(before);
});

test('late source evidence cannot pull attention back to the previously selected subject', async ({ page }) => {
  let release!: () => void;
  const pending = new Promise<void>(resolve => { release = resolve; });
  let requested!: () => void;
  const started = new Promise<void>(resolve => { requested = resolve; });
  await openFixture(page, { beforeContext: async path => { if (path === 'app/shipment.go') { requested(); await pending; } } });
  await selectPlace(page, /^Shipment$/);
  await showZones(page);
  await started;
  await selectPlace(page, /^Money$/);
  const completed = page.waitForResponse(response => new URL(response.url()).searchParams.get('path') === 'app/shipment.go');
  release();
  await completed;
  await expect(page.locator('#scene')).toHaveAttribute('data-selected-id', money.id);
  await openDetails(page);
  await expect(page.locator('#architecture-reading')).toContainText('core/money.go');
  await expect(page.locator('#architecture-reading')).not.toContainText('app/shipment.go');
  await expect(page.locator('#place-name')).toHaveText('Money');
});

test('automatic findings distinguish absent outcomes and repeated Baseline occurrences, then discard the Report on refresh', async ({ page }) => {
  test.setTimeout(60_000);
  const adopted = { kind: 'violation', ruleId: layerRule.id, path: 'app/shipment.go', line: 14, message: 'An existing upward dependency is acknowledged.', severity: 'error', status: 'baselined', fingerprint: 'same-occurrence-fingerprint' };
  const initial = checkGate(), refresh = checkGate();
  const fixture = await openFixture(page, { reports: [emptyReport, { ...emptyReport, diagnostics: [adopted, { ...adopted }] }], beforeCheck: async request => { if (request === 0) await initial.wait(); if (request === 2) await refresh.wait(); } });
  await selectPlace(page, /^Shipment$/);
  await initial.started;
  await expect(page.locator('#architecture-reading')).not.toBeVisible();
  await expect(page.locator('#scene [data-inspect-contract="address-known"]')).toBeVisible();
  await expect(page.locator('#scene .maquette-note[data-inspection-state="checking"]')).toBeVisible();
  const inspection = await openDetails(page, 'Inspection');
  await expect(inspection).toContainText('Evidence will appear here automatically.');
  await closeDetails(page);
  initial.release();
  await expect(page.locator(`#scene .study-label[data-subject-id="${shipment.id}"]`)).toHaveAttribute('data-inspection-state', 'reported');
  await expect(page.locator('#evidence-card')).not.toBeVisible();
  await page.locator('#repository-findings').click();
  const evidence = page.locator('#evidence-card');
  await expect(evidence).toContainText('Zero active findings does not establish that every Rule passed.');
  await expect(evidence).not.toContainText(/all Rules passed|architecture is clean/i);
  await page.locator('#close-evidence').click();
  fixture.changeRevision();
  await expect(page.locator('#scene [data-diagnostic-status="baselined"]')).toBeVisible();
  await expect(evidence).not.toBeVisible();
  await page.locator('#repository-findings').click();
  await expect(evidence.locator('[data-diagnostic-status="baselined"]')).toHaveCount(2);
  await evidence.locator('[data-report-filter="baselined"]').click();
  await expect(evidence.locator('[data-diagnostic-status="baselined"]')).toHaveCount(2);
  await expect(evidence).toContainText('Baseline adoption acknowledges findings; it does not repair them.');
  await page.locator('#refresh-repository').click();
  await refresh.started;
  await expect(evidence).toContainText('No Report yet');
  await expect(evidence.locator('[data-diagnostic-status="baselined"]')).toHaveCount(0);
  await page.locator('#close-evidence').click();
  await expect(page.locator('#scene [data-diagnostic-status="baselined"]')).toHaveCount(0);
  await expect(page.locator('#scene .maquette-note[data-inspection-state="checking"]')).toBeVisible();
  refresh.release();
  await expect(page.locator('#scene [data-diagnostic-status="baselined"]')).toBeVisible();
  await expect(evidence).not.toBeVisible();
  await expect(page.locator('#scene')).toHaveAttribute('data-selected-id', shipment.id);
  await page.locator('#repository-findings').click();
  await expect(evidence.locator('[data-diagnostic-status="baselined"]')).toHaveCount(2);
});

test('an automatic check finishing after evidence refresh cannot restore its stale Report', async ({ page }) => {
  const old = checkGate(), retry = checkGate();
  const oldReport = { ...emptyReport, diagnostics: [{ kind: 'violation', status: 'active', ruleId: layerRule.id, path: 'app/shipment.go', message: 'A finding from the old evidence generation.' }] };
  await openFixture(page, { reports: [oldReport, emptyReport], beforeCheck: request => request === 0 ? old.wait() : retry.wait() });
  try {
    await selectPlace(page, /^Shipment$/);
    await old.started;
    await expect(page.locator('#evidence-card')).not.toBeVisible();
    await page.locator('#repository-findings').click();
    await expect(page.locator('#evidence-card')).toContainText('No Report yet');
    const refreshed = page.waitForResponse('**/api/arclint/project');
    await page.locator('#refresh-repository').click();
    await refreshed;
    const completed = page.waitForResponse('**/api/arclint/check');
    old.release();
    await completed;
    // The evaluator has settled once Check code is available again. The stale
    // response must be absent from both the already-open Report and the scene.
    await expect(page.locator('#check-code-now')).toBeEnabled();
    await expect(page.locator('#evidence-card')).toContainText('No Report yet');
    await expect(page.locator('#evidence-card')).not.toContainText('A finding from the old evidence generation.');
    await expect(page.locator('#scene [data-diagnostic-status="active"]')).toHaveCount(0);
    await page.locator('#close-evidence').click();
    await page.locator('#repository-findings').click();
    await expect(page.locator('#evidence-card')).toContainText('No Report yet');
    await expect(page.locator('#evidence-card')).not.toContainText('A finding from the old evidence generation.');
  } finally { old.release(); retry.release(); }
});

test('automatic revision checks update the same scene without opening a Report or changing an unfinished definition', async ({ page }) => {
  const initial = checkGate(), changed = checkGate();
  const active = { ...emptyReport, diagnostics: [{ kind: 'violation', status: 'active', ruleId: layerRule.id, path: 'app/shipment.go', message: 'The initial code imports upward.' }] };
  const fixture = await openFixture(page, { reports: [active, emptyReport], beforeCheck: request => request === 0 ? initial.wait() : changed.wait() });
  try {
    await selectPlace(page, /^Shipment$/);
    await initial.started;
    await expect(page.locator('#scene .maquette-note[data-inspection-state="checking"]')).toBeVisible();
    await expect(page.locator('#scene [data-inspect-contract="address-known"]')).toBeVisible();
    initial.release();
    await expect(page.locator('#scene [data-diagnostic-status="active"]')).toBeVisible();
    await expect(page.locator('#evidence-card')).not.toBeVisible();
    await expect(page.locator('#architecture-reading')).not.toBeVisible();
    const before = await savedProject(page);
    await openEditor(page);
    const definition = page.getByRole('textbox', { name: 'Definition', exact: true });
    const draft = 'A definition still being written while the source changes.';
    await definition.fill(draft);
    await page.evaluate(() => {
      const evidence = document.querySelector<HTMLElement>('#evidence-card')!;
      (window as unknown as { reportOpened: boolean }).reportOpened = !evidence.hidden;
      new MutationObserver(() => { if (!evidence.hidden) (window as unknown as { reportOpened: boolean }).reportOpened = true; }).observe(evidence, { attributes: true, attributeFilter: ['hidden'] });
    });
    fixture.changeRevision();
    await changed.started;
    await expect(definition).toBeFocused();
    await expect(definition).toHaveValue(draft);
    changed.release();
    await expect(page.locator(`#scene .study-label[data-subject-id="${shipment.id}"]`)).toHaveAttribute('data-inspection-state', 'reported');
    await expect(page.locator('#scene [data-diagnostic-status="active"]')).toHaveCount(0);
    await expect(page.locator('#scene')).toHaveAttribute('data-selected-id', shipment.id);
    await expect(definition).toBeFocused();
    await expect(definition).toHaveValue(draft);
    expect(await page.evaluate(() => (window as unknown as { reportOpened: boolean }).reportOpened)).toBe(false);
    expect(await savedProject(page)).toEqual(before);
    expect(fixture.checkCount()).toBe(2);
    await closeWorkSurface(page);
    await expect(page.locator('#scene [data-inspect-contract="address-known"]')).toBeVisible();
    await openEditor(page);
    await expect(definition).toHaveValue(draft);
  } finally { initial.release(); changed.release(); }
});

test('an open Report refreshes automatically without losing its filter, search caret, or a later Baseline preview', async ({ page }) => {
  test.setTimeout(60_000);
  const changed = checkGate();
  const adopted = { kind: 'violation', status: 'baselined', ruleId: layerRule.id, path: 'app/shipment.go', message: 'Previously adopted import.' };
  const active = { ...adopted, status: 'active', message: 'An active import stays outside the Baseline filter.' };
  const initialReport = { ...emptyReport, diagnostics: [adopted, active] };
  const nextReport = { ...emptyReport, checkedAt: '2026-09-14T12:02:00Z', diagnostics: [adopted, { ...adopted, message: 'Newly returned adopted import.' }, active, { ...adopted, path: 'core/money.go', message: 'Another path stays outside the search.' }] };
  const preview: RepositoryBaselinePreview = { token: 'review-current-findings', action: 'capture', filename: '.arclint/baseline.json', findings: 1, rules: 1, report: { ...emptyReport, diagnostics: [active] }, expiresAt: '2026-09-14T12:30:00Z' };
  const fixture = await openFixture(page, { reports: [initialReport, nextReport, emptyReport], baselinePreview: preview, beforeCheck: async request => { if (request === 1) await changed.wait(); } });
  try {
    await selectPlace(page, /^Shipment$/);
    await expect(page.locator('#scene [data-diagnostic-status="active"]')).toBeVisible();
    await page.locator('#repository-findings').click();
    const evidence = page.locator('#evidence-card');
    await evidence.locator('[data-report-filter="baselined"]').click();
    const search = evidence.locator('#report-search');
    await search.fill('app/shipment.go');
    await search.evaluate(element => (element as HTMLInputElement).setSelectionRange(7, 7));
    await expect(evidence.locator('#report-records [data-diagnostic-status]')).toHaveCount(1);
    fixture.changeRevision();
    await changed.started;
    await expect(search).toBeFocused();
    await expect(search).toHaveValue('app/shipment.go');
    await expect(evidence.locator('#download-report')).toBeDisabled();
    await expect(evidence.locator('#review-baseline-adoption')).toBeDisabled();
    changed.release();
    await expect(evidence.locator('#report-records [data-diagnostic-status]')).toHaveCount(2);
    await expect(evidence.locator('#report-records')).toContainText('Newly returned adopted import.');
    await expect(evidence.locator('#report-records')).not.toContainText('An active import stays outside');
    await expect(evidence.locator('#report-records')).not.toContainText('Another path stays outside');
    await expect(evidence.locator('[data-report-filter="baselined"]')).toHaveAttribute('aria-pressed', 'true');
    await expect(search).toBeFocused();
    await expect(search).toHaveValue('app/shipment.go');
    expect(await search.evaluate(element => [(element as HTMLInputElement).selectionStart, (element as HTMLInputElement).selectionEnd])).toEqual([7, 7]);
    await expect(evidence.locator('#download-report')).toBeEnabled();
    await expect(evidence.locator('#review-baseline-adoption')).toBeEnabled();
    await expect(page.locator('#scene')).toHaveAttribute('data-selected-id', shipment.id);

    await evidence.locator('#review-baseline-adoption').click();
    await expect(evidence.getByRole('heading', { name: 'Review Baseline adoption', exact: true })).toBeVisible();
    await expect(evidence.locator('#apply-baseline-adoption')).toBeVisible();
    const checked = page.waitForResponse('**/api/arclint/check');
    fixture.changeRevision();
    expect((await checked).ok()).toBe(true);
    await expect(page.locator('#check-code-now')).toBeEnabled();
    await expect(page.locator(`#scene .study-label[data-subject-id="${shipment.id}"]`)).toHaveAttribute('data-inspection-state', 'reported');
    await expect(evidence.getByRole('heading', { name: 'Review Baseline adoption', exact: true })).toBeVisible();
    await expect(evidence.locator('#apply-baseline-adoption')).toBeVisible();
    await expect(evidence.locator('#report-search')).toHaveCount(0);
  } finally { changed.release(); }
});

for (const read of ['path', 'Rule'] as const) {
  test(`background inspection refreshes a foreground ${read} read without stranding it or reopening an abandoned reading`, async ({ page }) => {
    test.setTimeout(60_000);
    let held: ReturnType<typeof checkGate> | undefined;
    const gates: ReturnType<typeof checkGate>[] = [];
    async function holdNextRead() {
      const gate = held;
      held = undefined;
      if (gate) await gate.wait();
    }
    const fixture = await openFixture(page, {
      beforeContext: async path => { if (read === 'path' && path === 'app/shipment.go') await holdNextRead(); },
      beforeRule: async () => { if (read === 'Rule') await holdNextRead(); },
    });
    const evidence = page.locator('#evidence-card');
    async function openRead() {
      if (read === 'path') {
        await page.locator('#source-paths').click();
        await evidence.locator('#path-inspect-input').fill('app/shipment.go');
        await evidence.locator('#path-inspect-form').getByRole('button', { name: /Inspect/ }).click();
      } else await page.locator('#architecture-read-layer').click();
    }
    async function finishQuietRefresh() {
      const response = page.waitForResponse('**/api/arclint/check');
      fixture.changeRevision();
      expect((await response).ok()).toBe(true);
      await expect(page.locator('#check-code-now')).toBeEnabled();
      await expect(page.locator(`#scene .study-label[data-subject-id="${shipment.id}"]`)).toHaveAttribute('data-inspection-state', 'reported');
    }
    try {
      await selectPlace(page, /^Shipment$/);
      await expect(page.locator(`#scene .study-label[data-subject-id="${shipment.id}"]`)).toHaveAttribute('data-inspection-state', 'reported');
      if (read === 'Rule') {
        await showZones(page);
        await page.locator('#architecture-layer').selectOption(layerRule.id);
      }
      const first = checkGate(); gates.push(first); held = first;
      await openRead();
      await first.started;
      await finishQuietRefresh();
      first.release();
      if (read === 'path') {
        await expect(evidence).toContainText('app/shipment.go');
        await expect(evidence).toContainText('Zone memberships');
        await expect(evidence).toContainText('checkout');
        await expect(evidence).toContainText('audited');
      } else {
        await expect(evidence).toContainText(layerRule.id);
        await expect(evidence).toContainText(layerRule.rationale);
        await expect(evidence).toContainText('Analysis limits');
      }
      await expect(evidence).not.toContainText(/Inspecting code|Reading Rule|evidence changed while loading|could not be opened/i);
      await page.locator('#close-evidence').click();

      const abandoned = checkGate(); gates.push(abandoned); held = abandoned;
      await openRead();
      await abandoned.started;
      await finishQuietRefresh();
      if (read === 'path') await page.locator('#close-evidence').click();
      else {
        await evidence.locator('[data-governance-section="paths"]').click();
        await expect(evidence.getByRole('heading', { name: 'Code paths', exact: true })).toBeVisible();
      }
      const completion = page.waitForResponse(response => {
        const url = new URL(response.url());
        return read === 'path' ? url.pathname.endsWith('/context') && url.searchParams.get('path') === 'app/shipment.go' : url.pathname.endsWith('/rule');
      });
      abandoned.release();
      await (await completion).finished();
      await page.evaluate(() => new Promise<void>(resolve => requestAnimationFrame(() => resolve())));
      if (read === 'path') await expect(evidence).not.toBeVisible();
      else {
        await expect(evidence.getByRole('heading', { name: 'Code paths', exact: true })).toBeVisible();
        await expect(evidence).not.toContainText(layerRule.rationale);
      }
      await expect(page.locator('#scene')).toHaveAttribute('data-selected-id', shipment.id);
    } finally { for (const gate of gates) gate.release(); }
  });
}

test('automatic inspection preserves keyboard focus on a regenerated assertion checkpoint', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  const changed = checkGate();
  const fixture = await openFixture(page, { beforeCheck: async request => { if (request === 1) await changed.wait(); } });
  try {
    await selectPlace(page, /^Shipment$/);
    const owner = page.locator(`#scene .study-label[data-subject-id="${shipment.id}"]`);
    await expect(owner).toHaveAttribute('data-inspection-state', 'reported');
    const checkpoint = page.locator('#scene [data-inspect-contract="dispatch-ready"][data-contract-kind="assertion"]');
    await checkpoint.focus();
    await expect(checkpoint).toBeFocused();
    fixture.changeRevision();
    await changed.started;
    changed.release();
    await expect(owner).toHaveAttribute('data-inspection-state', 'reported');
    await expect(page.locator('#check-code-now')).toBeEnabled();
    await expect(checkpoint).toBeFocused();
    await page.keyboard.press('Enter');
    const reading = page.locator('#architecture-reading');
    await expect(reading).toBeVisible();
    await expect(reading.locator('.architecture-contract h3')).toHaveText('dispatch-ready');
    await expect(reading.getByText('After Dispatch', { exact: true })).toBeVisible();
    await expect(reading.getByText('Every item is ready before dispatch.', { exact: true })).toBeVisible();
    await expect(page.locator('#scene')).toHaveAttribute('data-selected-id', shipment.id);
    await expect(page.locator('#inspector')).not.toBeVisible();
  } finally { changed.release(); }
});

test('the mobile keyboard journey preserves place and unfinished writing across Zones and evidence', async ({ page }, testInfo) => {
  test.setTimeout(60_000);
  await page.setViewportSize({ width: 390, height: 844 });
  await openFixture(page);
  await tabActivate(page, page.locator(`#scene [data-subject-id="${fulfilment.id}"]`));
  await tabActivate(page, page.locator(`#scene [data-subject-id="${shipment.id}"]`));
  await tabActivate(page, page.locator('#scene [data-inspect-contract="dispatch-ready"]'));
  await expect(page.locator('#architecture-reading')).toContainText('Every item is ready before dispatch.');
  await openEditor(page);
  const draft = 'A dispatch definition awaiting stakeholder review.';
  await page.getByRole('textbox', { name: 'Definition', exact: true }).fill(draft);
  await closeWorkSurface(page);
  await closeDetails(page);
  await tabActivate(page, page.locator('#zone-overlay'));
  await expect(page.locator('#zone-overlay')).toHaveAttribute('aria-pressed', 'true');
  await expect(page.locator('#architecture-reading')).not.toBeVisible();
  expect(await page.evaluate(() => {
    const summary = document.querySelector('.place-orientation')!.getBoundingClientRect();
    const toolbar = document.querySelector('#architecture-tools')!.getBoundingClientRect();
    return !summary.width || !summary.height || toolbar.left >= summary.right || toolbar.right <= summary.left || toolbar.top >= summary.bottom || toolbar.bottom <= summary.top;
  }), 'Overlay controls do not cover the selected place or definition').toBe(true);
  await tabActivate(page, page.locator('#scene [data-inspect-path="app/shipment.go"]').first());
  await expect(page.locator('#evidence-card')).toContainText('Zone memberships');
  await tabActivate(page, page.locator('#close-evidence'));
  await expect(page.locator('#scene')).toHaveAttribute('data-selected-id', shipment.id);
  await tabActivate(page, page.locator('#architecture-open-reading'));
  await openDetails(page, 'Files & Zones');
  const readingToggle = page.locator('#architecture-reading .architecture-reading-toggle');
  await expect(page.locator('#architecture-reading-body')).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  for (const selector of ['#zone-overlay', '#architecture-open-reading', '#ascend', '#governing-rules', '#source-paths', '#check-code-now']) {
    const button = page.locator(selector);
    await expect(button).toBeVisible();
    expect(await button.evaluate(element => {
      const rect = element.getBoundingClientRect();
      const top = document.elementFromPoint(rect.x + rect.width / 2, rect.y + rect.height / 2);
      return (top === element || element.contains(top)) && rect.width >= 24 && rect.height >= 24;
    }), `${selector} has an unobscured target`).toBe(true);
  }
  await expect(page.locator('.study-scene')).toHaveAttribute('data-camera-state', 'settled');
  expect(await page.evaluate(() => {
    const heading = document.querySelector('#place-name')!.getBoundingClientRect();
    return [...document.querySelectorAll<HTMLElement>('#scene .maquette-note')].every(note => {
      const style = getComputedStyle(note);
      if (style.display === 'none' || style.visibility === 'hidden' || Number(style.opacity) === 0) return true;
      const bounds = note.getBoundingClientRect();
      return bounds.right <= heading.left || bounds.left >= heading.right || bounds.bottom <= heading.top || bounds.top >= heading.bottom;
    });
  }), 'Expanded reading keeps the place heading clear of map annotations').toBe(true);
  await page.screenshot({ path: testInfo.outputPath('mobile-structure-return.png'), fullPage: true });
  await readingToggle.click();
  await expect(page.locator('#architecture-reading')).not.toBeVisible();
  await expect(page.locator('.study-scene')).toHaveAttribute('data-camera-state', 'settled');
  await page.screenshot({ path: testInfo.outputPath('mobile-structure-folded.png'), fullPage: true });
  await showZones(page, false);
  await openEditor(page);
  await expect(page.getByRole('textbox', { name: 'Definition', exact: true })).toHaveValue(draft);
  expect(exportDomainYaml(await savedProject(page))).toEqual(exportDomainYaml(model));
});
