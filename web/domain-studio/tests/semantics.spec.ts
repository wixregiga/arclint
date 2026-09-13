import { expect, test, type Page } from '@playwright/test';

const library = `version: 1
project: Library
description: A general catalog and lending domain.
contexts:
  catalog:
    definition: The titles and editions held by a library.
    aggregates:
      Book:
        definition: A catalog title with a stable identity.
        identity: BookID
        invariants:
          title-present: Book has a nonempty title.
        assertions:
          publication-ready:
            on: Publish
            statement: Book has a publication date.
        entities:
          Edition:
            definition: A published edition of a Book.
            identity: EditionID
    value_objects:
      BookID:
        definition: The stable identity of a catalog title.
      ISBN:
        definition: An international identifier of a published edition.
        invariants:
          normalized: ISBN contains only normalized identifier characters.
  lending:
    definition: Lending catalog titles to library members.
    value_objects:
      LoanID:
        definition: The identity of a lending record.
relations:
  - from: catalog
    to: lending
    kind: conformist
    description: Lending uses the catalog language for published titles.
`;

async function loadLibrary(page: Page) {
  await page.goto('/');
  await page.locator('#import-file').setInputFiles({
    name: 'domain.arclint.yaml', mimeType: 'application/yaml', buffer: Buffer.from(library),
  });
  await expect(page.locator('#project-name')).toHaveText('Library');
}

async function savedProject(page: Page) {
  return page.evaluate(() => JSON.parse(localStorage.getItem('arclint.domain-studio.v1')!).project);
}

async function askWorkbench(page: Page, input: string) {
  await page.locator('#workbench-input').fill(input);
  await page.locator('#workbench-submit').click();
}

test('plan objects can each be selected without another object blocking their target', async ({ page }) => {
  test.setTimeout(60_000);
  await loadLibrary(page);
  const project = await savedProject(page);
  await page.locator('#view-plan').click();
  for (const object of [...project.contexts, ...project.concepts]) {
    const target = page.locator(`.plan-object[data-model-select="${object.id}"]`);
    await target.click();
    await expect(page.locator('#inspector').getByRole('textbox', { name: 'Name', exact: true })).toHaveValue(object.name);
    await page.getByRole('button', { name: 'Clear selection', exact: true }).click();
  }
});

test('Site, Plan, and Matrix preserve model identities and positions', async ({ page }, testInfo) => {
  await loadLibrary(page);
  const original = await savedProject(page);
  for (const representation of ['plan', 'matrix', 'site']) {
    await page.locator(`#view-${representation}`).click();
    await expect(page.locator(`#view-${representation}`)).toHaveAttribute('aria-pressed', 'true');
    expect(await savedProject(page)).toEqual(original);
  }
  await page.locator('#view-plan').click();
  await page.screenshot({ path: testInfo.outputPath('library-plan.png'), fullPage: true });
});

test('directional relationships expose their actual name in Plan and Matrix', async ({ page }) => {
  await loadLibrary(page);
  for (const representation of ['plan', 'matrix']) {
    await page.locator(`#view-${representation}`).click();
    const connection = page.locator(`[data-${representation}-relation="yaml:relations/0"]`);
    await expect(connection).toBeVisible();
    await expect(connection).toHaveAccessibleName(/catalog.*conformist.*lending/i);
    const target = await connection.boundingBox();
    expect(target?.width).toBeGreaterThanOrEqual(24);
    expect(target?.height).toBeGreaterThanOrEqual(24);
    await connection.click();
    await expect(page.locator('#inspector')).toContainText('catalog');
    await expect(page.locator('#inspector')).toContainText('lending');
    await expect(page.getByRole('textbox', { name: 'Relationship', exact: true })).toHaveValue('conformist');
    await page.getByRole('button', { name: 'Clear selection', exact: true }).click();
  }
});

test('exact-name workbench selection exposes canonical invariant and assertion meaning', async ({ page }) => {
  await loadLibrary(page);
  await askWorkbench(page, 'Book');
  const inspector = page.locator('#inspector');
  await expect(inspector.getByRole('textbox', { name: 'Name', exact: true })).toHaveValue('Book');
  await expect(inspector).toContainText('Book has a nonempty title.');
  const assertions = inspector.locator('details').filter({ has: page.locator('summary').filter({ hasText: /^Assertions/ }) });
  if (!await assertions.evaluate(node => (node as HTMLDetailsElement).open)) await assertions.locator('summary').click();
  await expect(assertions).toContainText('Publish');
  await expect(assertions).toContainText('Book has a publication date.');
});

test('repository actions return actual context, layers, CLI results, and visible path errors', async ({ page }, testInfo) => {
  test.setTimeout(120_000);
  await page.goto('/');
  await page.locator('#open-repository').click();
  await expect(page.locator('#project-name')).toHaveText('arclint');
  await askWorkbench(page, 'internal/domain/rule/root.go');
  const evidence = page.locator('#evidence-card');
  await expect(evidence).toBeVisible();
  await expect(evidence).toContainText('internal/domain/rule/root.go');
  await expect(evidence).toContainText(/rule/i);
  await page.locator('#open-architecture').click();
  await expect(evidence).toContainText('application');
  await expect(evidence).toContainText('domain');
  const zoneResponse = page.waitForResponse(response => new URL(response.url()).searchParams.get('zone') === 'domain');
  await evidence.locator('[data-inspect-zone="domain"]').click();
  const zone = await zoneResponse;
  expect(zone.ok()).toBe(true);
  expect((await zone.json()).zone).toBe('domain');
  await expect(evidence).toContainText(/domain/i);
  const checkResponse = page.waitForResponse(response => response.url().endsWith('/api/arclint/check') && response.request().method() === 'POST', { timeout: 90_000 });
  await page.locator('#run-repository-check').click();
  const response = await checkResponse;
  expect(response.ok()).toBe(true);
  const result = await response.json();
  await expect(evidence).toContainText(/arclint check/i, { timeout: 90_000 });
  await expect(evidence).toContainText(new RegExp(`exit\\s*:?\\s*${result.exitCode}`, 'i'));
  const diagnostic = result.diagnostics.find((item: { kind: string }) => item.kind === 'violation');
  if (diagnostic) {
    await expect(evidence).toContainText(diagnostic.ruleId);
    await expect(evidence).toContainText(diagnostic.status);
  }
  await page.screenshot({ path: testInfo.outputPath('repository-evidence.png'), fullPage: true });
  const errorResponse = page.waitForResponse(response => response.url().includes('/api/arclint/context?'));
  await askWorkbench(page, '../outside.go');
  expect((await errorResponse).status()).toBe(400);
  await expect(evidence).toContainText(/INVALID_PATH|without traversal|relative repository path/i);
});

for (const viewport of [{ width: 626, height: 766 }, { width: 942, height: 766 }]) {
  test(`semantic selection remains readable and reachable at ${viewport.width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize(viewport);
    await loadLibrary(page);
    const input = await page.locator('#workbench-input').boundingBox();
    const submit = await page.locator('#workbench-submit').boundingBox();
    expect(Math.abs((input!.y + input!.height / 2) - (submit!.y + submit!.height / 2))).toBeLessThan(6);
    await page.locator('#view-plan').click();
    const connection = page.locator('[data-plan-relation="yaml:relations/0"]');
    await connection.click();
    await expect(page.getByRole('textbox', { name: 'Relationship', exact: true })).toHaveValue('conformist');
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.screenshot({ path: testInfo.outputPath(`library-plan-${viewport.width}.png`), fullPage: true });
  });
}

test('large matrices page rows and columns without changing the model', async ({ page }) => {
  await page.goto('/');
  const project = {
    version: 1, name: 'Large domain', description: 'Matrix navigation fixture',
    contexts: [{ id: 'area', name: 'area', description: 'Recorded terms', color: '#637966', position: [0, 0, 0] }],
    concepts: Array.from({ length: 40 }, (_, i) => ({ id: `term-${i}`, name: `Term ${i}`, definition: 'An unresolved term', kind: 'unclassified', contextId: 'area', invariants: [], position: [i % 8 * 8, 2, Math.floor(i / 8) * 8] })),
    relationships: [{ id: 'last-link', source: 'term-39', target: 'term-38', label: 'supplies' }],
  };
  await page.locator('#import-file').setInputFiles({name:'large.json',mimeType:'application/json',buffer:Buffer.from(JSON.stringify(project))});
  await expect(page.locator('#project-name')).toHaveText('Large domain');
  await page.locator('#view-matrix').click();
  expect(await page.locator('#projection td').count()).toBeLessThanOrEqual(1024);
  await page.getByRole('button',{name:'Next rows',exact:true}).click();
  await page.getByRole('button',{name:'Next columns',exact:true}).click();
  await expect(page.locator('[data-matrix-relation="last-link"]')).toHaveText('supplies');
  expect(await savedProject(page)).toEqual(project);
  await expect(page.locator('#overview')).toBeHidden();
  await expect(page.locator('#zoom-in')).toBeHidden();
  await page.locator('#view-site').click();
  await expect(page.locator('#overview')).toBeVisible();
});
