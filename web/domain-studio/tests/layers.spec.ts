import { expect, test, type Page } from '@playwright/test';
import { closeTools, openEditor, openNavigator, selectPlace, toolButton, toolId, savedProject } from './studio.helpers';

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
  await loadLargeLibrary(page);
  await expect(page.locator('#scene')).toHaveAttribute('data-depth', 'world');
  await expect(page.locator('#scene [data-subject-id]:visible')).toHaveCount(3);
  await expect(page.locator('#scene .domain-regions')).toHaveCount(1);
  await expect(page.locator('#scene [data-subject-kind="context"]')).toHaveCount(3);
  await expect(page.locator('#scene').getByRole('button', { name: /^Book/ })).not.toBeVisible();
  for (const selector of ['#inspector', '#tools-drawer', '#evidence-card', '#architecture-reading', '#workbench-input', '#projection', '#model-tree']) {
    await expect(page.locator(selector)).not.toBeVisible();
  }
  await expect(page.locator('#lens-meaning, #lens-structure, #lens-inspection, #view-prev, #view-next, #view-page')).toHaveCount(0);
  await expect(page.locator('#scene').getByText(/No located anchors/i)).toHaveCount(0);
  await expect(page.locator('#zone-overlay')).toBeVisible();
  await expect(page.locator('#open-index')).toBeVisible();
  await expect(page.locator('#tools-toggle')).toBeVisible();
  await expect(page.locator('#keeper-action')).toHaveAttribute('data-keeper', 'Onyx');
  await expect(page.locator('#keeper-action .onyx-avatar')).toHaveCount(1);
  await expect(page.locator('#scene .onyx-avatar')).toHaveCount(0);
  await expect(page.locator('.study-scene')).toHaveAttribute('data-camera-state', 'settled');
  await page.screenshot({ path: testInfo.outputPath('quiet-world-990.png'), fullPage: true });
});

test('one click enters a context, named groups reveal every entry, and Find reaches another group', async ({ page }, testInfo) => {
  await loadLargeLibrary(page);
  await page.locator('#scene [data-subject-id="catalog"]').click();
  await expect(page.locator('#scene')).toHaveAttribute('data-depth', 'context');
  await expect(page.locator('#scene')).toHaveAttribute('data-scope', 'catalog');
  await expect(page.locator('.study-scene')).toHaveAttribute('data-context-enclosure', 'none');
  await expect(page.locator('#inspector')).not.toBeVisible();
  await expect(page.locator('#architecture-reading')).not.toBeVisible();
  await expect(page.locator('#keeper-action')).toHaveAttribute('data-keeper', 'Onyx');
  await expect(page.locator('#scene [data-subject-id="book-1"]')).toBeVisible();
  await expect(page.locator('.study-scene')).toHaveAttribute('data-camera-state', 'settled');
  const visible = new Set<string | null>();
  const groupOptions = await page.locator('#view-group option').evaluateAll(options => options.map(option => ({ value: (option as HTMLOptionElement).value, label: option.textContent ?? '' })));
  expect(groupOptions.length).toBeGreaterThan(1);
  for (const group of groupOptions) {
    expect(group.label).toMatch(/Book/);
    expect(group.label).not.toMatch(/^\d+\s*(?:of|\/)\s*\d+$/);
    await page.locator('#view-group').selectOption(group.value);
    await expectBoundedSubjects(page);
    (await subjects(page)).forEach(id => visible.add(id));
  }
  for (let index = 1; index <= 18; index++) expect(visible.has(`book-${index}`)).toBe(true);
  await expect(page.locator('.study-scene')).toHaveAttribute('data-camera-state', 'settled');
  await page.screenshot({ path: testInfo.outputPath('catalog-final-page.png'), fullPage: true });
  await page.locator('#view-group').selectOption('0');
  await openNavigator(page);
  await page.locator('#search').fill('Book 18');
  await page.getByTestId('model-tree').getByRole('button', { name: /^Book 18/ }).click();
  await expect(page.locator('#scene')).toHaveAttribute('data-depth', 'detail');
  await expect(page.locator('#scene')).toHaveAttribute('data-selected-id', 'book-18');
  await expect(page.locator('.study-scene')).toHaveAttribute('data-context-enclosure', 'none');
  await expectBoundedSubjects(page);
  await expect(page.locator('#keeper-action')).toHaveAttribute('data-keeper', 'Onyx');
  await expect(page.locator('.study-scene')).toHaveAttribute('data-camera-state', 'settled');
  await expect(page.locator('#place-name')).toHaveText('Book 18');
  await expect(page.locator('#scene [data-inspect-contract]')).toBeVisible();
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

test('Up restores the context group and camera that preceded a drill down', async ({ page }) => {
  await loadLargeLibrary(page);
  await page.locator('#scene [data-subject-id="catalog"]').click();
  await page.locator('#view-group').selectOption('2');
  const scene = page.locator('.study-scene');
  await expect(scene).toHaveAttribute('data-camera-state', 'settled');
  const initialDistance = Number(await scene.getAttribute('data-camera-distance'));
  await page.getByRole('button', { name: 'Zoom out', exact: true }).click();
  await expect.poll(async () => Number(await scene.getAttribute('data-camera-distance'))).toBeGreaterThan(initialDistance * 1.1);
  await expect(scene).toHaveAttribute('data-camera-state', 'settled');
  const before = await subjects(page);
  const distance = Number(await scene.getAttribute('data-camera-distance'));
  await page.locator('#scene [data-subject-id="book-17"]').click();
  await expect(page.locator('#place-name')).toHaveText('Book 17');
  await page.locator('#ascend').click();
  await expect(page.locator('#view-group')).toHaveValue('2');
  await expect(scene).toHaveAttribute('data-camera-state', 'settled');
  expect(await subjects(page)).toEqual(before);
  expect(Number(await scene.getAttribute('data-camera-distance'))).toBeCloseTo(distance, 0);
});

test('the same Onyx avatar opens contextual support, and explicit editing preserves the selected place', async ({ page }) => {
  await loadLargeLibrary(page);
  await selectPlace(page, /^Book 01/);
  await expect(page.locator('#inspector')).not.toBeVisible();
  await page.locator('#keeper-action').click();
  await expect(page.locator('#onyx-support')).toBeVisible();
  await expect(page.locator('#onyx-place')).toContainText('Book 01');
  await expect(page.locator('#inspector')).not.toBeVisible();
  await page.locator('#onyx-define').click();
  await expect(page.locator('#inspector')).toBeVisible();
  await expect(page.getByRole('textbox', { name: 'Name', exact: true })).toHaveValue('Book 01');
  await page.getByRole('textbox', { name: 'Definition', exact: true }).fill('A named title that readers can discover.');
  await page.getByRole('button', { name: 'Save definition', exact: true }).click();
  await page.locator('#clear-selection').click();
  await expect(page.locator('#inspector')).not.toBeVisible();
  await expect(page.locator('#scene')).toHaveAttribute('data-depth', 'detail');
  await expect(page.locator('#scene')).toHaveAttribute('data-selected-id', 'book-1');
  await openEditor(page);
  await expect(page.getByRole('textbox', { name: 'Definition', exact: true })).toHaveValue('A named title that readers can discover.');
});

test('related endpoints, the Zone overlay, and model snapshot ghosts share the eight-subject budget', async ({ page }) => {
  await loadLargeLibrary(page);
  await selectPlace(page, /^Book 01/);
  await expectBoundedSubjects(page);
  const original = await savedProject(page);
  for (const shown of [true, false]) {
    await page.locator('#zone-overlay').click();
    await expect(page.locator('#zone-overlay')).toHaveAttribute('aria-pressed', String(shown));
    await expect(page.locator('#evidence-card')).not.toBeVisible();
    await expect(page.locator('#architecture-reading')).not.toBeVisible();
    await expect(page.locator('#scene [data-inspect-contract]')).toBeVisible();
    await expect(page.locator('#scene .study-label[data-subject-id="book-1"]')).toHaveAttribute('data-inspection-state', /.+/);
    await expectBoundedSubjects(page);
    expect(await savedProject(page)).toEqual(original);
  }
  await toolButton(page, 'Model snapshot');
  await page.getByRole('button', { name: 'Capture model snapshot', exact: true }).click();
  await page.getByRole('dialog').getByRole('textbox', { name: 'Name', exact: true }).fill('Before clarification');
  await page.getByRole('button', { name: 'Save snapshot', exact: true }).click();
  await selectPlace(page, /^Book 01/);
  await openEditor(page);
  await page.getByRole('textbox', { name: 'Definition', exact: true }).fill('A clarified library title.');
  await page.getByRole('button', { name: 'Save definition', exact: true }).click();
  await page.locator('#clear-selection').click();
  await toolButton(page, 'Model snapshot');
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

test('focused editing and evidence suppress background creation and undo shortcuts', async ({ page }) => {
  await loadLargeLibrary(page);
  await selectPlace(page, /^Book 01/);
  await openEditor(page);
  await page.getByRole('textbox', { name: 'Definition', exact: true }).fill('A title with an intentionally saved clarification.');
  await page.getByRole('button', { name: 'Save definition', exact: true }).click();
  const saved = await savedProject(page);
  await page.locator('#clear-selection').focus();
  await page.keyboard.press('c');
  await expect(page.getByRole('dialog')).not.toBeVisible();
  await page.keyboard.press('Control+z');
  expect(await savedProject(page)).toEqual(saved);
  await expect(page.locator('#inspector')).toBeVisible();
  await toolId(page, '#open-architecture');
  await expect(page.locator('#evidence-card')).toContainText('domain');
  await page.locator('#close-evidence').focus();
  await page.keyboard.press('c');
  await expect(page.getByRole('dialog')).not.toBeVisible();
  await page.keyboard.press('Control+z');
  expect(await savedProject(page)).toEqual(saved);
  await expect(page.locator('#evidence-card')).toBeVisible();
});

test('following an external neighbor keeps Back in the originating context while Find changes home', async ({ page }) => {
  await loadLargeLibrary(page);
  await selectPlace(page, /^Book 01/);
  const groups = await page.locator('#view-group option').evaluateAll(options => options.map(option => (option as HTMLOptionElement).value));
  for (const group of groups) {
    await page.locator('#view-group').selectOption(group);
    await expectBoundedSubjects(page);
    if (await page.locator('#scene [data-subject-id="loan"]').isVisible()) break;
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
