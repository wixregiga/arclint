import { expect, test } from '@playwright/test';
import { readFile } from 'node:fs/promises';
import { createExampleProject } from '../src/domain';

test.beforeEach(async ({page}) => {
  await page.addInitScript(project => localStorage.setItem('arclint.domain-studio.v1',JSON.stringify({project,baseline:null,patterns:[]})),createExampleProject());
});

import { openNavigator, selectConcept, selectPlace, workspaceAction, creationAction, toolId, toolButton } from './studio.helpers';

test('inspects imported Pattern rules and prepares a scoped AI request', async ({ page }, testInfo) => {
  await page.goto('/');
  await toolButton(page, 'Patterns');
  await page.locator('#pattern-file').setInputFiles({
    name: 'pattern.yaml',
    mimeType: 'application/yaml',
    buffer: Buffer.from('pattern:\n  namespace: library\n  name: catalog\n  version: 1.0.0\nrules:\n  named-books:\n    rationale: Every book has a meaningful name.\n'),
  });
  await page.locator('summary').filter({ hasText: 'library/catalog@1.0.0' }).click();
  await expect(page.getByText('library/catalog:named-books', { exact: true })).toBeVisible();
  await expect(page.getByText(/they are not executed here/)).toBeVisible();
  await selectConcept(page, /^Skill/);
  await page.getByRole('button', { name: 'Prepare AI request', exact: true }).click();
  await expect(page.getByRole('textbox', { name: 'Request', exact: true })).toHaveValue(/Skill/);
  const downloadPromise = page.waitForEvent('download');
  await page.getByRole('button', { name: 'Download request', exact: true }).click();
  const download = await downloadPromise;
  const path = testInfo.outputPath(download.suggestedFilename());
  await download.saveAs(path);
  expect(await readFile(path, 'utf8')).toContain('Skill');
});

test('edits an unrelated domain, restores it, and compares a model snapshot', async ({ page }, testInfo) => {
  test.setTimeout(60_000);
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.goto('/');
  await workspaceAction(page, 'New domain');
  let dialog = page.getByRole('dialog');
  await dialog.getByLabel('Name', { exact: true }).fill('Library');
  await dialog.getByLabel('Description', { exact: true }).fill('A library domain unrelated to skills or harnesses.');
  await dialog.getByRole('button', { name: 'Create domain', exact: true }).click();

  await creationAction(page, 'Add context');
  dialog = page.getByRole('dialog');
  await dialog.getByLabel('Name', { exact: true }).fill('Catalog');
  await dialog.getByLabel('Definition', { exact: true }).fill('Describes books and their authors.');
  await dialog.getByRole('button', { name: 'Create context', exact: true }).click();

  for (const [name, definition] of [['Book', 'A title available in the library.'], ['Author', 'The credited writer of a book.']]) {
    await creationAction(page, 'Add concept');
    dialog = page.getByRole('dialog');
    await dialog.getByLabel('Name', { exact: true }).fill(name);
    await dialog.getByLabel('Definition', { exact: true }).fill(definition);
    await dialog.getByRole('combobox', { name: 'Context', exact: true }).selectOption({ label: 'Catalog' });
    await dialog.getByRole('button', { name: 'Save definition', exact: true }).click();
  }
  await creationAction(page, 'Connect');
  dialog = page.getByRole('dialog');
  await dialog.getByRole('combobox', { name: 'From', exact: true }).selectOption({ label: 'Book' });
  await dialog.getByRole('combobox', { name: 'To', exact: true }).selectOption({ label: 'Author' });
  await dialog.getByLabel('Relationship', { exact: true }).fill('written by');
  await dialog.getByRole('button', { name: 'Create relationship', exact: true }).click();

  await page.reload();
  await openNavigator(page);
  await expect(page.getByTestId('model-tree')).toContainText('Catalog');
  await expect(page.getByTestId('model-tree')).toContainText('Book');
  await expect(page.getByTestId('model-tree')).toContainText('Author');
  await page.locator('#close-index').click();

  await toolButton(page, 'Model snapshot');
  await page.getByRole('button', { name: 'Capture model snapshot', exact: true }).click();
  dialog = page.getByRole('dialog');
  await dialog.getByLabel('Name', { exact: true }).fill('Initial catalog');
  await dialog.getByRole('button', { name: 'Save snapshot', exact: true }).click();
  await selectConcept(page, /Book/);
  await page.getByRole('textbox', { name: 'Definition', exact: true }).fill('A cataloged title with a stable identity.');
  await page.getByRole('button', { name: 'Save definition', exact: true }).click();
  await toolButton(page, 'Model snapshot');
  await expect(page.getByText('Initial catalog', { exact: false }).first()).toBeVisible();
  await expect(page.getByText(/changed/i).first()).toBeVisible();
  await toolButton(page, 'Undo');
  await toolButton(page, 'Model snapshot');
  await expect(page.getByText('Your model matches this snapshot.', { exact: true })).toBeVisible();
  await toolButton(page, 'Redo');
  await toolButton(page, 'Model snapshot');
  await expect(page.getByText(/changed/i).first()).toBeVisible();
  await page.reload();
  await toolButton(page, 'Model snapshot');
  await expect(page.getByText('Initial catalog', { exact: false }).first()).toBeVisible();
  await page.screenshot({ path: testInfo.outputPath('library-baseline.png'), fullPage: true });

  await workspaceAction(page, 'Export');
  const downloadPromise = page.waitForEvent('download');
  await page.getByRole('button', { name: /Download workspace/ }).click();
  const download = await downloadPromise;
  const downloadPath = testInfo.outputPath(download.suggestedFilename());
  await download.saveAs(downloadPath);
  const saved = await readFile(downloadPath, 'utf8');
  expect(saved).toContain('A cataloged title with a stable identity.');
  expect(saved).toContain('written by');
  const envelope = JSON.parse(saved);
  expect(envelope.version).toBe(2);
  expect(envelope.model.name).toBe('Library');
  expect(envelope.modelSnapshot.name).toBe('Initial catalog');
  expect(envelope.model.concepts.every((concept: object) => !('position' in concept))).toBe(true);
  expect(Object.keys(envelope.layout)).toHaveLength(3);

  await page.evaluate(() => localStorage.clear());
  await page.reload();
  await page.locator('#import-file').setInputFiles(downloadPath);
  await openNavigator(page);
  await expect(page.getByTestId('model-tree')).toContainText('Book');
  await expect(page.getByTestId('model-tree')).toContainText('Author');
  await selectConcept(page, /Book/);
  await expect(page.getByRole('textbox', { name: 'Definition', exact: true })).toHaveValue('A cataloged title with a stable identity.');
  expect(errors).toEqual([]);
});

test('renders the spatial editor without browser errors', async ({ page }, testInfo) => {
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  page.on('console', message => { if (message.type() === 'error') errors.push(message.text()); });
  await page.goto('/');
  await expect(page.locator('canvas')).toBeVisible();
  await expect(page.locator('#inspector')).not.toBeVisible();
  await expect(page.getByTestId('model-tree')).not.toBeVisible();
  const bounds = await page.getByTestId('scene').boundingBox();
  const viewport = page.viewportSize()!;
  expect(bounds).toEqual({ x: 0, y: 0, width: viewport.width, height: viewport.height });
  await expect(page.locator('#tools-drawer')).not.toBeVisible();
  await page.screenshot({ path: testInfo.outputPath('studio-desktop.png'), fullPage: true });
  expect(errors).toEqual([]);
});

test('enters an actual context and ascends to the realm', async ({ page }, testInfo) => {
  await page.goto('/');
  await page.locator('#import-file').setInputFiles({
    name: 'library.json',
    mimeType: 'application/json',
    buffer: Buffer.from(JSON.stringify({
      version: 1, name: 'Library', description: 'A general domain.',
      contexts: [{ id: 'catalog', name: 'Catalog', description: 'Library titles.', color: '#455c71', position: [0, 0, 0] }],
      concepts: [{ id: 'book', name: 'Book', definition: 'A catalog title.', kind: 'unclassified', contextId: 'catalog', invariants: [], position: [0, 1, 0] }],
      relationships: [],
    })),
  });
  await expect(page.locator('#ascend')).toBeDisabled();
  await expect(page.getByTestId('scene')).toHaveAttribute('data-scope', 'realm');
  await selectPlace(page, /^Catalog/);
  await expect(page.locator('#ascend')).toBeEnabled();
  await expect(page.getByTestId('scene')).toHaveAttribute('data-scope', 'catalog');
  await page.screenshot({ path: testInfo.outputPath('library-context.png'), fullPage: true });
  await page.locator('#ascend').click();
  await expect(page.locator('#ascend')).toBeDisabled();
  await expect(page.getByTestId('scene')).toHaveAttribute('data-scope', 'realm');
  await expect(page.locator('#inspector')).not.toBeVisible();
  await expect(page.getByTestId('model-tree')).not.toBeVisible();
});

for (const viewport of [{ width: 768, height: 1024 }, { width: 390, height: 844 }]) {
  test(`keeps scene and editing controls usable at ${viewport.width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize(viewport);
    await page.goto('/');
    await expect(page.locator('canvas')).toBeVisible();
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth);
    expect(overflow).toBe(false);
    await selectConcept(page, /^Skill/);
    await expect(page.getByRole('button', { name: 'Save definition', exact: true })).toBeVisible();
    await page.getByRole('textbox', { name: 'Definition', exact: true }).fill('A reusable capability, edited in the compact inspector.');
    await page.getByRole('button', { name: 'Save definition', exact: true }).click();
    await expect(page.getByRole('textbox', { name: 'Definition', exact: true })).toHaveValue('A reusable capability, edited in the compact inspector.');
    await page.screenshot({ path: testInfo.outputPath(`studio-${viewport.width}-inspector.png`), fullPage: true });
    await creationAction(page, 'Add context');
    await expect(page.getByRole('dialog')).toBeVisible();
    await expect(page.getByRole('dialog').getByLabel('Name', { exact: true })).toBeVisible();
    await page.screenshot({ path: testInfo.outputPath(`studio-${viewport.width}.png`), fullPage: true });
  });
}
