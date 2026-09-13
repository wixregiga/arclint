import { expect, type Page } from '@playwright/test';
import { assembleProject } from '../src/workspace';
import type { DomainProject } from '../src/contracts';

export async function savedProject(page: Page): Promise<DomainProject> {
  const saved = await page.evaluate(() => JSON.parse(localStorage.getItem('arclint.domain-studio.v2') ?? localStorage.getItem('arclint.domain-studio.v1')!));
  return saved.version === 2 ? assembleProject(saved) : saved.project;
}

export async function openTools(page: Page) {
  if (!await page.locator('#tools-drawer').isVisible()) {
    await closeWorkSurface(page);
    await page.locator('#tools-toggle').click();
  }
}

export async function closeWorkSurface(page: Page) {
  if (await page.locator('#onyx-support').isVisible()) await page.locator('#close-onyx').click();
  if (await page.locator('#inspector').isVisible()) {
    if (await page.locator('#clear-selection').isVisible()) await page.locator('#clear-selection').click();
    else if (await page.locator('#close-sheet').isVisible()) await page.locator('#close-sheet').click();
  }
  if (await page.locator('#evidence-card').isVisible()) await page.locator('#close-evidence').click();
}

export async function closeTools(page: Page) {
  if (await page.locator('#tools-drawer').isVisible()) await page.locator('#close-tools').click();
}

export async function toolId(page: Page, selector: string) {
  await openTools(page);
  await page.locator(selector).click();
}

export async function toolButton(page: Page, name: string) {
  await openTools(page);
  await page.getByRole('button', { name, exact: true }).click();
  if (name === 'Undo' || name === 'Redo') await closeTools(page);
}

export async function openNavigator(page: Page) {
  await closeTools(page);
  await closeWorkSurface(page);
  if (!await page.getByTestId('model-tree').isVisible()) await page.locator('#open-index').click();
}

export async function selectPlace(page: Page, name: RegExp) {
  await openNavigator(page);
  await page.getByTestId('model-tree').getByRole('button', { name }).click();
  await expect(page.getByTestId('model-tree')).not.toBeVisible();
}

export async function openEditor(page: Page) {
  await closeTools(page);
  if (!await page.locator('#inspector').isVisible()) await page.locator('#open-editor').click();
  await expect(page.locator('#inspector')).toBeVisible();
}

export async function selectConcept(page: Page, name: RegExp) {
  await selectPlace(page, name);
  await openEditor(page);
}

export async function workspaceAction(page: Page, name: string) {
  await toolButton(page, 'Workspace menu');
  await page.getByRole('button', { name, exact: true }).click();
}

export async function creationAction(page: Page, name: string, kind = 'unclassified') {
  await toolId(page, name === 'Connect' ? '#connect' : name === 'Add context' ? '#add-context' : '#add-concept');
  if (name !== 'Connect' && name !== 'Add context') await page.locator(`[data-create-kind="${kind}"]`).click();
}
