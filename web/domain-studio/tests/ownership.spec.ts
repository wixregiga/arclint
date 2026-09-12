import { expect, test } from '@playwright/test';
import { readFile } from 'node:fs/promises';
import { parse } from 'yaml';
import { createEmptyProject } from '../src/domain';
import type { DomainProject } from '../src/contracts';

function library(): DomainProject {
  return { ...createEmptyProject(), name: 'Library', description: 'A general library domain.', contexts: [{ id: 'catalog', name: 'catalog', description: 'Library titles and editions.', color: '#66d8df', position: [0, 0, 0] }] };
}
async function load(page: import('@playwright/test').Page, project: DomainProject) {
  await page.goto('/');
  await page.locator('#import-file').setInputFiles({ name: 'library.json', mimeType: 'application/json', buffer: Buffer.from(JSON.stringify(project)) });
  await expect(page.locator('#project-name')).toHaveText('Library');
}

test('creates aggregate and member through the UI and exports canonical ownership', async ({ page }, testInfo) => {
  await load(page, library());
  await page.locator('#add-concept').click();
  let dialog = page.getByRole('dialog');
  await dialog.getByRole('textbox', { name: 'Name', exact: true }).fill('Book');
  await dialog.getByRole('textbox', { name: 'Definition', exact: true }).fill('A catalog title with a stable identity.');
  await dialog.getByRole('combobox', { name: 'Kind', exact: true }).selectOption('aggregate');
  await dialog.getByRole('textbox', { name: 'Identity', exact: true }).fill('BookID');
  await dialog.getByRole('textbox', { name: 'Aliases', exact: true }).fill('Title');
  await dialog.getByRole('textbox', { name: 'Invariants', exact: true }).fill('Book has at least one edition.');
  await dialog.getByRole('button', { name: 'Create concept', exact: true }).click();
  await page.locator('#add-concept').click();
  dialog = page.getByRole('dialog');
  await dialog.getByRole('textbox', { name: 'Name', exact: true }).fill('Edition');
  await dialog.getByRole('textbox', { name: 'Definition', exact: true }).fill('A published edition belonging to a Book.');
  await dialog.getByRole('combobox', { name: 'Kind', exact: true }).selectOption('entity');
  await dialog.getByRole('textbox', { name: 'Identity', exact: true }).fill('EditionID');
  await dialog.getByRole('combobox', { name: 'Owning aggregate', exact: true }).selectOption({ label: 'Book' });
  await dialog.getByRole('button', { name: 'Create concept', exact: true }).click();
  await page.getByRole('button', { name: 'Export', exact: true }).click();
  const pending = page.waitForEvent('download');
  await page.getByRole('button', { name: /Download domain YAML/ }).click();
  const download = await pending;
  const path = testInfo.outputPath('domain.arclint.yaml');
  await download.saveAs(path);
  const yaml = parse(await readFile(path, 'utf8'));
  expect(yaml.contexts.catalog.aggregates.Book.identity).toBe('BookID');
  expect(yaml.contexts.catalog.aggregates.Book.entities.Edition.identity).toBe('EditionID');
  expect(yaml.contexts.catalog.aggregates.Book.aliases).toEqual(['Title']);
  await page.locator('#import-file').setInputFiles(path);
  await page.getByTestId('model-tree').getByRole('button', { name: /^Edition/ }).click();
  await expect(page.getByRole('combobox', { name: 'Owning aggregate', exact: true })).toHaveValue('yaml:contexts/catalog/aggregates/Book');
  // Deleting an aggregate keeps the surviving member as an explicit unresolved draft.
  await page.getByTestId('model-tree').getByRole('button', { name: /^Book/ }).click();
  await page.getByRole('button', { name: 'Delete concept', exact: true }).click();
  await page.getByRole('dialog').getByRole('button', { name: 'Delete', exact: true }).click();
  await page.getByTestId('model-tree').getByRole('button', { name: /^Edition/ }).click();
  await expect(page.getByRole('combobox', { name: 'Owning aggregate', exact: true })).toHaveValue('');
  await expect(page.locator('#inspector')).not.toContainText('owns member');
});

test('saving another field preserves multiline invariants and comma-containing aliases', async ({ page }) => {
  const model = library();
  model.concepts.push({ id: 'book', contextId: 'catalog', name: 'Book', definition: 'A title.', kind: 'aggregate', identity: 'BookID', aliases: ['Title, cataloged', 'Work'], invariants: ['Book keeps its identity\nacross every edition.', 'Book has a title.'], position: [0, 1, 0] });
  await load(page, model);
  await page.getByTestId('model-tree').getByRole('button', { name: /^Book/ }).click();
  await page.getByRole('textbox', { name: 'Definition', exact: true }).fill('A title in the library catalog.');
  await page.getByRole('button', { name: 'Save concept', exact: true }).click();
  const saved = await page.evaluate(() => JSON.parse(localStorage.getItem('arclint.domain-studio.v1')!).project);
  expect(saved.concepts[0].aliases).toEqual(model.concepts[0].aliases);
  expect(saved.concepts[0].invariants).toEqual(model.concepts[0].invariants);
});
