import { expect, test } from '@playwright/test';
import { readFile } from 'node:fs/promises';

test('inspects imported Pattern rules and prepares a scoped AI request', async ({ page }, testInfo) => {
  await page.goto('/');
  await page.getByRole('button', { name: 'Patterns', exact: true }).click();
  await page.locator('#pattern-file').setInputFiles({
    name: 'pattern.yaml',
    mimeType: 'application/yaml',
    buffer: Buffer.from('pattern:\n  namespace: library\n  name: catalog\n  version: 1.0.0\nrules:\n  named-books:\n    rationale: Every book has a meaningful name.\n'),
  });
  await page.locator('summary').filter({ hasText: 'library/catalog@1.0.0' }).click();
  await expect(page.getByText('library/catalog:named-books', { exact: true })).toBeVisible();
  await expect(page.getByText(/they are not executed here/)).toBeVisible();
  await page.getByRole('button', { name: 'Domain', exact: true }).click();
  await page.getByTestId('model-tree').getByRole('button', { name: /^Skill/ }).click();
  await page.getByRole('button', { name: 'Prepare AI request', exact: true }).click();
  await expect(page.getByRole('textbox', { name: 'Request', exact: true })).toHaveValue(/Skill/);
  const downloadPromise = page.waitForEvent('download');
  await page.getByRole('button', { name: 'Download request', exact: true }).click();
  const download = await downloadPromise;
  const path = testInfo.outputPath(download.suggestedFilename());
  await download.saveAs(path);
  expect(await readFile(path, 'utf8')).toContain('Skill');
});

test('edits an unrelated domain, restores it, and compares a baseline', async ({ page }, testInfo) => {
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.goto('/');
  await page.getByRole('button', { name: 'New domain', exact: true }).click();
  let dialog = page.getByRole('dialog');
  await dialog.getByLabel('Name', { exact: true }).fill('Library');
  await dialog.getByLabel('Description', { exact: true }).fill('A library domain unrelated to skills or harnesses.');
  await dialog.getByRole('button', { name: 'Create domain', exact: true }).click();

  await page.getByRole('button', { name: 'Add context', exact: true }).click();
  dialog = page.getByRole('dialog');
  await dialog.getByLabel('Name', { exact: true }).fill('Catalog');
  await dialog.getByLabel('Definition', { exact: true }).fill('Describes books and their authors.');
  await dialog.getByRole('button', { name: 'Create context', exact: true }).click();

  for (const [name, definition] of [['Book', 'A title available in the library.'], ['Author', 'The credited writer of a book.']]) {
    await page.locator('#add-concept').click();
    dialog = page.getByRole('dialog');
    await dialog.getByLabel('Name', { exact: true }).fill(name);
    await dialog.getByLabel('Definition', { exact: true }).fill(definition);
    await dialog.getByRole('combobox', { name: 'Context', exact: true }).selectOption({ label: 'Catalog' });
    await dialog.getByRole('button', { name: 'Create concept', exact: true }).click();
  }
  await page.locator('#connect').click();
  dialog = page.getByRole('dialog');
  await dialog.getByRole('combobox', { name: 'From', exact: true }).selectOption({ label: 'Book' });
  await dialog.getByRole('combobox', { name: 'To', exact: true }).selectOption({ label: 'Author' });
  await dialog.getByLabel('Relationship', { exact: true }).fill('written by');
  await dialog.getByRole('button', { name: 'Create relationship', exact: true }).click();

  await page.reload();
  await expect(page.getByTestId('model-tree')).toContainText('Catalog');
  await expect(page.getByTestId('model-tree')).toContainText('Book');
  await expect(page.getByTestId('model-tree')).toContainText('Author');

  await page.getByRole('button', { name: 'Baseline', exact: true }).click();
  await page.getByRole('button', { name: 'Capture baseline', exact: true }).click();
  dialog = page.getByRole('dialog');
  await dialog.getByLabel('Name', { exact: true }).fill('Initial catalog');
  await dialog.getByRole('button', { name: 'Save baseline', exact: true }).click();
  await page.getByRole('button', { name: 'Domain', exact: true }).click();
  await page.getByTestId('model-tree').getByRole('button', { name: /Book/ }).click();
  await page.getByRole('textbox', { name: 'Definition', exact: true }).fill('A cataloged title with a stable identity.');
  await page.getByRole('button', { name: 'Save concept', exact: true }).click();
  await page.getByRole('button', { name: 'Baseline', exact: true }).click();
  await expect(page.getByText('Initial catalog', { exact: false }).first()).toBeVisible();
  await expect(page.getByText(/changed/i).first()).toBeVisible();
  await page.getByRole('button', { name: 'Undo', exact: true }).click();
  await expect(page.getByText('Your model matches this baseline.', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Redo', exact: true }).click();
  await expect(page.getByText(/changed/i).first()).toBeVisible();
  await page.reload();
  await page.getByRole('button', { name: 'Baseline', exact: true }).click();
  await expect(page.getByText('Initial catalog', { exact: false }).first()).toBeVisible();
  await page.screenshot({ path: testInfo.outputPath('library-baseline.png'), fullPage: true });

  await page.getByRole('button', { name: 'Export', exact: true }).click();
  const downloadPromise = page.waitForEvent('download');
  await page.getByRole('button', { name: /Download workspace/ }).click();
  const download = await downloadPromise;
  const downloadPath = testInfo.outputPath(download.suggestedFilename());
  await download.saveAs(downloadPath);
  const saved = await readFile(downloadPath, 'utf8');
  expect(saved).toContain('A cataloged title with a stable identity.');
  expect(saved).toContain('written by');

  await page.evaluate(() => localStorage.clear());
  await page.reload();
  await page.locator('#import-file').setInputFiles(downloadPath);
  await expect(page.getByTestId('model-tree')).toContainText('Book');
  await expect(page.getByTestId('model-tree')).toContainText('Author');
  await page.getByTestId('model-tree').getByRole('button', { name: /Book/ }).click();
  await expect(page.getByRole('textbox', { name: 'Definition', exact: true })).toHaveValue('A cataloged title with a stable identity.');
  expect(errors).toEqual([]);
});

test('renders the spatial editor without browser errors', async ({ page }, testInfo) => {
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.goto('/');
  await expect(page.locator('canvas')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Add context', exact: true })).toBeVisible();
  await expect(page.locator('#add-concept')).toBeVisible();
  await page.screenshot({ path: testInfo.outputPath('studio-desktop.png'), fullPage: true });
  expect(errors).toEqual([]);
});

for (const viewport of [{ width: 768, height: 1024 }, { width: 390, height: 844 }]) {
  test(`keeps scene and editing controls usable at ${viewport.width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize(viewport);
    await page.goto('/');
    await expect(page.locator('canvas')).toBeVisible();
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth);
    expect(overflow).toBe(false);
    await page.getByTestId('model-tree').getByRole('button', { name: /^Skill/ }).click();
    await expect(page.getByRole('button', { name: 'Save concept', exact: true })).toBeVisible();
    await page.getByRole('textbox', { name: 'Definition', exact: true }).fill('A reusable capability, edited in the compact inspector.');
    await page.getByRole('button', { name: 'Save concept', exact: true }).click();
    await expect(page.getByRole('textbox', { name: 'Definition', exact: true })).toHaveValue('A reusable capability, edited in the compact inspector.');
    await page.screenshot({ path: testInfo.outputPath(`studio-${viewport.width}-inspector.png`), fullPage: true });
    await page.getByRole('button', { name: 'Add context', exact: true }).click();
    await expect(page.getByRole('dialog')).toBeVisible();
    await expect(page.getByRole('dialog').getByLabel('Name', { exact: true })).toBeVisible();
    await page.screenshot({ path: testInfo.outputPath(`studio-${viewport.width}.png`), fullPage: true });
  });
}
