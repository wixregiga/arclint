import { expect, test, type Page } from '@playwright/test';
import { closeTools, openEditor, openNavigator, selectPlace, toolButton, toolId } from './studio.helpers';

const largeLibrary = {
  version: 1, name: 'Library', description: 'A domain large enough to require deliberate layers.',
  contexts: [
    { id: 'catalog', name: 'Catalog', description: 'Catalog titles.', color: '#475C57', position: [-30, 0, 0] },
    { id: 'lending', name: 'Lending', description: 'Loans of catalog titles.', color: '#867554', position: [30, 0, 0] },
    { id: 'membership', name: 'Membership', description: 'Registered readers.', color: '#5C657E', position: [0, 0, 35] },
  ],
  concepts: [
    ...Array.from({ length: 18 }, (_, index) => ({ id: `book-${index + 1}`, name: `Book ${String(index + 1).padStart(2, '0')}`, definition: 'A title in the library catalog.', kind: 'value_object', contextId: 'catalog', invariants: ['A title has a name.'], position: [-30 + index % 6 * 5, 1, Math.floor(index / 6) * 5] })),
    { id: 'loan', name: 'Loan', definition: 'The lending of a title.', kind: 'unclassified', contextId: 'lending', invariants: [], position: [30, 1, 0] },
    { id: 'reader', name: 'Reader', definition: 'A registered library reader.', kind: 'unclassified', contextId: 'membership', invariants: [], position: [0, 1, 35] },
  ],
  relationships: [
    ...Array.from({ length: 17 }, (_, index) => ({ id: `related-${index + 2}`, source: 'book-1', target: `book-${index + 2}`, label: 'related title' })),
    { id: 'lent-title', source: 'book-1', target: 'loan', label: 'appears in' },
    { id: 'read-title', source: 'reader', target: 'book-1', label: 'reads' },
  ],
};

async function loadLargeLibrary(page: Page) {
  await page.goto('/');
  await page.locator('#import-file').setInputFiles({ name: 'library.json', mimeType: 'application/json', buffer: Buffer.from(JSON.stringify(largeLibrary)) });
  await expect(page.locator('#project-name')).toHaveText('Library');
}

async function subjects(page: Page) {
  return page.locator('#scene [data-subject-id]:visible').evaluateAll(nodes => nodes.map(node => node.getAttribute('data-subject-id')));
}

async function expectBoundedSubjects(page: Page) {
  await expect.poll(async () => (await subjects(page)).length).toBeGreaterThan(0);
  expect((await subjects(page)).length).toBeLessThanOrEqual(8);
}

test('first view is a quiet world of contexts, with no descendant labels or work surfaces', async ({ page }, testInfo) => {
  await page.setViewportSize({ width: 990, height: 623 });
  await page.goto('/');
  await expect(page.locator('#scene')).toHaveAttribute('data-depth', 'world');
  await expect(page.locator('#scene [data-subject-id]:visible')).toHaveCount(3);
  await expect(page.locator('#scene').getByRole('button', { name: /^Skill/ })).not.toBeVisible();
  for (const selector of ['#inspector', '#tools-drawer', '#evidence-card', '#workbench-input', '#projection', '#model-tree']) {
    await expect(page.locator(selector)).not.toBeVisible();
  }
  await expect(page.locator('#open-index')).toBeVisible();
  await expect(page.locator('#tools-toggle')).toBeVisible();
  await expect(page.locator('#keeper-action')).toHaveAttribute('data-keeper', 'Onyx');
  await expect(page.locator('.study-scene')).toHaveAttribute('data-camera-state', 'settled');
  await page.screenshot({ path: testInfo.outputPath('quiet-world-990.png'), fullPage: true });
});

test('one click descends, paging reveals the complete context, and Find reaches an off-page subject', async ({ page }, testInfo) => {
  await loadLargeLibrary(page);
  await page.locator('#scene [data-subject-id="catalog"]').click();
  await expect(page.locator('#scene')).toHaveAttribute('data-depth', 'context');
  await expect(page.locator('#scene')).toHaveAttribute('data-scope', 'catalog');
  await expect(page.locator('#inspector')).not.toBeVisible();
  await expect(page.locator('#keeper-action')).toHaveAttribute('data-keeper', 'Steward');
  await expect(page.locator('#scene [data-subject-id="book-1"]')).toBeVisible();
  await expect(page.locator('.study-scene')).toHaveAttribute('data-camera-state', 'settled');
  const contextDistance = Number(await page.locator('.study-scene').getAttribute('data-camera-distance'));
  const visible = new Set<string | null>();
  for (let pageNumber = 0; pageNumber < 6; pageNumber++) {
    await expectBoundedSubjects(page);
    (await subjects(page)).forEach(id => visible.add(id));
    if (await page.locator('#view-next').isDisabled()) break;
    const previous = await subjects(page);
    await page.locator('#view-next').click();
    await expect.poll(() => subjects(page)).not.toEqual(previous);
  }
  for (let index = 1; index <= 18; index++) expect(visible.has(`book-${index}`)).toBe(true);
  await expect(page.locator('.study-scene')).toHaveAttribute('data-camera-state', 'settled');
  await page.screenshot({ path: testInfo.outputPath('catalog-final-page.png'), fullPage: true });
  await page.locator('#view-prev').click();
  await openNavigator(page);
  await page.locator('#search').fill('Book 18');
  await page.getByTestId('model-tree').getByRole('button', { name: /^Book 18/ }).click();
  await expect(page.locator('#scene')).toHaveAttribute('data-depth', 'detail');
  await expect(page.locator('#scene')).toHaveAttribute('data-selected-id', 'book-18');
  await expectBoundedSubjects(page);
  await expect(page.locator('#keeper-action')).toHaveAttribute('data-keeper', 'Inspector');
  await expect(page.locator('.study-scene')).toHaveAttribute('data-camera-state', 'settled');
  expect(Number(await page.locator('.study-scene').getAttribute('data-camera-distance'))).toBeLessThan(contextDistance);
  const selectedBounds = await page.locator('#scene [data-subject-id="book-18"]').boundingBox();
  const viewport = page.viewportSize()!;
  expect(selectedBounds!.x).toBeGreaterThanOrEqual(12);
  expect(selectedBounds!.y).toBeGreaterThanOrEqual(12);
  expect(selectedBounds!.x + selectedBounds!.width).toBeLessThanOrEqual(viewport.width - 12);
  expect(selectedBounds!.y + selectedBounds!.height).toBeLessThanOrEqual(viewport.height - 12);
  await page.screenshot({ path: testInfo.outputPath('book-detail.png'), fullPage: true });
  await page.locator('#ascend').click();
  await expect(page.locator('#scene')).toHaveAttribute('data-depth', 'context');
  await page.locator('#ascend').click();
  await expect(page.locator('#scene')).toHaveAttribute('data-depth', 'world');
});

test('Edit is explicit, its close preserves the place, and the keeper invokes that real action', async ({ page }) => {
  await loadLargeLibrary(page);
  await selectPlace(page, /^Book 01/);
  await expect(page.locator('#inspector')).not.toBeVisible();
  await page.locator('#keeper-action').click();
  await expect(page.locator('#inspector')).toBeVisible();
  await expect(page.getByRole('textbox', { name: 'Name', exact: true })).toHaveValue('Book 01');
  await page.getByRole('textbox', { name: 'Definition', exact: true }).fill('A named title that readers can discover.');
  await page.getByRole('button', { name: 'Save concept', exact: true }).click();
  await page.locator('#clear-selection').click();
  await expect(page.locator('#inspector')).not.toBeVisible();
  await expect(page.locator('#scene')).toHaveAttribute('data-depth', 'detail');
  await expect(page.locator('#scene')).toHaveAttribute('data-selected-id', 'book-1');
  await openEditor(page);
  await expect(page.getByRole('textbox', { name: 'Definition', exact: true })).toHaveValue('A named title that readers can discover.');
});

test('related endpoints, lenses, and baseline ghosts share the eight-subject budget', async ({ page }) => {
  await loadLargeLibrary(page);
  await selectPlace(page, /^Book 01/);
  await expectBoundedSubjects(page);
  const original = await page.evaluate(() => JSON.parse(localStorage.getItem('arclint.domain-studio.v1')!).project);
  for (const lens of ['#lens-governance', '#lens-meaning']) {
    await page.locator(lens).click();
    await expectBoundedSubjects(page);
    expect(await page.evaluate(() => JSON.parse(localStorage.getItem('arclint.domain-studio.v1')!).project)).toEqual(original);
  }
  await toolButton(page, 'Baseline');
  await page.getByRole('button', { name: 'Capture baseline', exact: true }).click();
  await page.getByRole('dialog').getByRole('textbox', { name: 'Name', exact: true }).fill('Before clarification');
  await page.getByRole('button', { name: 'Save baseline', exact: true }).click();
  await selectPlace(page, /^Book 01/);
  await openEditor(page);
  await page.getByRole('textbox', { name: 'Definition', exact: true }).fill('A clarified library title.');
  await page.getByRole('button', { name: 'Save concept', exact: true }).click();
  await page.locator('#clear-selection').click();
  await toolButton(page, 'Baseline');
  await closeTools(page);
  await expect(page.locator('#inspector')).toContainText('Changed definition.');
  await page.locator('#close-sheet').click();
  await expect(page.locator('#inspector')).not.toBeVisible();
  await expect(page.locator('#scene')).toHaveAttribute('data-comparison', 'baseline');
  await expect(page.locator('#exit-comparison')).toBeVisible();
  expect(Number(await page.locator('.study-scene').getAttribute('data-visible-subjects'))).toBeLessThanOrEqual(8);
  await page.locator('#scene [data-subject-id="book-2"]').click();
  await expect(page.locator('#scene')).toHaveAttribute('data-selected-id', 'book-2');
  await expect(page.locator('#scene')).toHaveAttribute('data-comparison', 'baseline');
  await page.locator('#ascend').click();
  await expect(page.locator('#scene')).toHaveAttribute('data-depth', 'context');
  await expect(page.locator('#scene')).toHaveAttribute('data-comparison', 'baseline');
  await page.locator('#exit-comparison').click();
  await expect(page.locator('#scene')).not.toHaveAttribute('data-comparison', 'baseline');
});

test('fullscreen editing and evidence suppress background creation and undo shortcuts', async ({ page }) => {
  await loadLargeLibrary(page);
  await selectPlace(page, /^Book 01/);
  await openEditor(page);
  await page.getByRole('textbox', { name: 'Definition', exact: true }).fill('A title with an intentionally saved clarification.');
  await page.getByRole('button', { name: 'Save concept', exact: true }).click();
  const saved = await page.evaluate(() => JSON.parse(localStorage.getItem('arclint.domain-studio.v1')!).project);
  await page.locator('#clear-selection').focus();
  await page.keyboard.press('c');
  await expect(page.getByRole('dialog')).not.toBeVisible();
  await page.keyboard.press('Control+z');
  expect(await page.evaluate(() => JSON.parse(localStorage.getItem('arclint.domain-studio.v1')!).project)).toEqual(saved);
  await expect(page.locator('#inspector')).toBeVisible();
  await toolId(page, '#open-architecture');
  await expect(page.locator('#evidence-card')).toContainText('domain');
  await page.locator('#close-evidence').focus();
  await page.keyboard.press('c');
  await expect(page.getByRole('dialog')).not.toBeVisible();
  await page.keyboard.press('Control+z');
  expect(await page.evaluate(() => JSON.parse(localStorage.getItem('arclint.domain-studio.v1')!).project)).toEqual(saved);
  await expect(page.locator('#evidence-card')).toBeVisible();
});

test('following an external neighbor keeps Back in the originating context while Find changes home', async ({ page }) => {
  await loadLargeLibrary(page);
  await selectPlace(page, /^Book 01/);
  for (let index = 0; index < 6 && !await page.locator('#scene [data-subject-id="loan"]').isVisible(); index++) {
    await page.locator('#view-next').click();
  }
  await expect(page.locator('#scene [data-subject-id="loan"]')).toBeVisible();
  await expect(page.locator('.study-scene')).toHaveAttribute('data-camera-state', 'settled');
  await page.locator('#scene [data-subject-id="loan"]').click();
  await expect(page.locator('#scene')).toHaveAttribute('data-selected-id', 'loan');
  await expect(page.locator('#scene')).toHaveAttribute('data-scope', 'catalog');
  await page.locator('#ascend').click();
  await expect(page.locator('#scene')).toHaveAttribute('data-depth', 'context');
  await expect(page.locator('#scene')).toHaveAttribute('data-scope', 'catalog');
  await selectPlace(page, /^Loan/);
  await expect(page.locator('#scene')).toHaveAttribute('data-scope', 'lending');
});
