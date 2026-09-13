import { expect, test, type Page } from '@playwright/test';
import { readFileSync } from 'node:fs';
import { importProject, exportDomainYaml } from '../src/serialization';
import { assembleProject } from '../src/workspace';
import { savedProject, closeWorkSurface } from './studio.helpers';

const canonical = readFileSync(new URL('../../../domain.arclint.yaml', import.meta.url),'utf8');
const initial = importProject(canonical);
const ruleContext = initial.contexts.find(c => c.name === 'rule')!.id;
const ruleRoot = initial.concepts.find(c => c.kind === 'aggregate')!.id;
async function openRealModel(page: Page, model = initial) {
  await page.addInitScript(project => {
    if (!localStorage.getItem('arclint.domain-studio.v2')) localStorage.setItem('arclint.domain-studio.v1',JSON.stringify({project,baseline:{name:'Before editing',capturedAt:'2026-09-13T00:00:00Z',project},patterns:[]}));
  }, model);
  await page.goto('/');
  await expect(page.locator('#project-name')).toHaveText('arclint');
}
async function openRule(page: Page) {
  await page.locator('#representation-table').click();
  await page.locator(`[data-table-select="${ruleContext}"]`).click();
  await page.locator(`[data-table-edit="${ruleRoot}"]`).click();
}

test('unfinished definition survives presentation, governance, navigation and reload without changing the model', async ({page}) => {
  await openRealModel(page); await openRule(page);
  const text = '  A definition still being discussed.\nKeep this second line exactly.  ';
  const definition = page.locator('#concept-form [name="definition"]');
  await definition.fill(text);
  await page.locator('#representation-spatial').click();
  await expect(definition).toHaveValue(text);
  expect(exportDomainYaml(await savedProject(page))).toEqual(exportDomainYaml(initial));
  await page.locator('#source-paths').click();
  await expect(page.locator('#evidence-card')).toBeVisible();
  await page.locator('#close-evidence').click();
  await expect(definition).toHaveValue(text);
  await page.reload();
  await page.locator('#open-editor').click();
  await expect(definition).toHaveValue(text);
  await page.locator('#concept-form button[type="submit"]').click();
  expect((await savedProject(page)).concepts.find(c => c.id === ruleRoot)!.definition).toBe(text.trim());
  await page.locator('#representation-table').click();
  await expect(definition).toHaveValue(text.trim());
});

test('typing first creates a recoverable unassigned draft and then a real value object in an imported domain', async ({page}) => {
  await openRealModel(page);
  const text = '  A revision identifies one published policy.\nIts value remains stable.  ';
  await page.locator('#build-input').fill(text);
  await page.reload();
  await expect(page.locator('#build-input')).toHaveValue(text);
  await page.locator('#build-draft').click();
  await page.locator('#cancel-dialog').click();
  let saved = await page.evaluate(() => JSON.parse(localStorage.getItem('arclint.domain-studio.v2')!));
  expect(saved.notebook[0].text).toBe(text);
  expect(assembleProject(saved).concepts).toEqual(initial.concepts);
  await page.locator('#open-notebook').click();
  await page.locator('[data-assign-draft]').click();
  await page.locator('[data-create-kind="value_object"]').click();
  const form = page.locator('#dialog-form');
  await expect(form.locator('[name="definition"]')).toHaveValue(text);
  await form.locator('[name="name"]').fill('PolicyRevision');
  await form.locator('[name="contextId"]').selectOption(ruleContext);
  await form.locator('button[type="submit"]').click();
  saved = await page.evaluate(() => JSON.parse(localStorage.getItem('arclint.domain-studio.v2')!));
  expect(saved.notebook).toHaveLength(0);
  const project = assembleProject(saved);
  expect(project.concepts.find(c => c.name === 'PolicyRevision')?.kind).toBe('value_object');
  const written = importProject(exportDomainYaml(project));
  expect(written.concepts.find(c => c.name === 'PolicyRevision')?.definition).toBe(text.trim());
  expect(written.concepts.find(c => c.kind === 'aggregate')?.invariants).toEqual(initial.concepts.find(c => c.kind === 'aggregate')?.invariants);
  await page.locator('#tools-toggle').click();
  await page.locator('#undo').click();
  saved = await page.evaluate(() => JSON.parse(localStorage.getItem('arclint.domain-studio.v2')!));
  expect(assembleProject(saved).concepts.some(c => c.name === 'PolicyRevision')).toBe(false);
  expect(saved.notebook[0].text).toBe(text);
  await expect(page.locator('#build-input')).toHaveValue(text);
  await page.locator('#redo').click();
  saved = await page.evaluate(() => JSON.parse(localStorage.getItem('arclint.domain-studio.v2')!));
  expect(assembleProject(saved).concepts.some(c => c.name === 'PolicyRevision')).toBe(true);
  expect(saved.notebook).toHaveLength(0);
});

test('operation assertions are authored on their aggregate and persist in canonical export', async ({page}) => {
  await openRealModel(page); await openRule(page);
  await page.locator('#add-assertion').click();
  const form = page.locator('#dialog-form');
  await form.locator('[name="key"]').fill('review-complete');
  await form.locator('[name="on"]').fill('Review');
  await form.locator('[name="statement"]').fill('Every accepted change has a rationale.');
  await form.locator('button[type="submit"]').click();
  await expect(page.locator('[data-edit-assertion="review-complete"]')).toContainText('After Review');
  const result = importProject(exportDomainYaml(await savedProject(page)));
  expect(result.concepts.find(c => c.kind === 'aggregate')!.assertions).toContainEqual({key:'review-complete',on:'Review',statement:'Every accepted change has a rationale.'});
});

test('context navigation, the single support avatar, and all primary controls remain usable on mobile', async ({page}) => {
  await page.setViewportSize({width:390,height:844});
  await openRealModel(page);
  await page.locator('#representation-table').click();
  await page.locator(`[data-table-select="${ruleContext}"]`).click();
  await expect(page.locator('#breadcrumbs')).toContainText('rule');
  await page.locator('#representation-spatial').click();
  await expect(page.locator('.study-label[data-subject-id]')).toHaveCount(8);
  await expect(page.locator('.onyx-avatar')).toHaveCount(1);
  await page.locator('#keeper-action').click();
  await page.locator('#onyx-input').fill('Where am I?');
  await page.locator('#onyx-form button').click();
  await expect(page.locator('#onyx-conversation')).toContainText('inside rule');
  await page.locator('#close-onyx').click();
  await page.locator('#realm-location').click();
  await expect(page.locator('#scene')).toHaveAttribute('data-depth','world');
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  for (const selector of ['#governing-rules','#source-paths','#check-code-now','#build-draft','#map-plus','#keeper-action']) {
    await expect(page.locator(selector)).toBeVisible();
    const clickable = await page.locator(selector).evaluate(element => { const r=element.getBoundingClientRect(); const top=document.elementFromPoint(r.x+r.width/2,r.y+r.height/2); return top === element || element.contains(top); });
    expect(clickable,selector+' is not obscured').toBe(true);
  }
});

test('invalid domain contract changes remain recoverable and do not commit partial mutations', async ({page}) => {
  await openRealModel(page); await openRule(page);
  const before = await savedProject(page);
  await page.locator('#add-assertion').click();
  const form = page.locator('#dialog-form');
  await form.locator('[name="key"]').fill('Not A Canonical Key');
  await form.locator('[name="on"]').fill('Review');
  await form.locator('[name="statement"]').fill('A promise.');
  await form.locator('button[type="submit"]').click();
  await expect(form.locator('.form-error')).toContainText('Assertion keys');
  expect(await savedProject(page)).toEqual(before);
  await page.locator('#cancel-dialog').click();
  await closeWorkSurface(page);
});


test('a rejected assertion for another aggregate leaves selection and subsequent editing on the original owner', async ({page}) => {
  const model = structuredClone(initial);
  model.concepts.push({id:'review-root',name:'Review',definition:'A review of proposed changes.',kind:'aggregate',identity:'ReviewID',contextId:ruleContext,invariants:[],position:[4,1,4]});
  await openRealModel(page,model); await openRule(page);
  await page.locator('#add-assertion').click();
  const form = page.locator('#dialog-form');
  await form.locator('[name="ownerId"]').selectOption('review-root');
  await form.locator('[name="key"]').fill('Invalid Key');
  await form.locator('[name="on"]').fill('Accept');
  await form.locator('[name="statement"]').fill('A rejected draft.');
  await form.locator('button[type="submit"]').click();
  await expect(form.locator('.form-error')).toContainText('Assertion keys');
  await page.locator('#cancel-dialog').click();
  await expect(page.locator('#concept-form [name="name"]')).toHaveValue('Rule');
  await page.locator('#concept-form [name="definition"]').fill('The original aggregate is still selected.');
  await page.locator('#concept-form button[type="submit"]').click();
  const saved = await savedProject(page);
  expect(saved.concepts.find(c => c.id === ruleRoot)!.definition).toBe('The original aggregate is still selected.');
  expect(saved.concepts.find(c => c.id === 'review-root')!.definition).toBe('A review of proposed changes.');
});


test('assigning an older notebook draft preserves unrelated unsent Build text', async ({page}) => {
  await openRealModel(page);
  const older = 'A stable revision of a policy.';
  const unsent = '  A separate unresolved thought.\nKeep working on this.  ';
  await page.locator('#build-input').fill(older);
  await page.locator('#build-draft').click();
  await page.locator('#cancel-dialog').click();
  await page.locator('#build-input').fill(unsent);
  await page.locator('#open-notebook').click();
  await page.locator('[data-assign-draft]').click();
  await page.locator('[data-create-kind="value_object"]').click();
  const form = page.locator('#dialog-form');
  await form.locator('[name="name"]').fill('PolicyRevision');
  await form.locator('[name="contextId"]').selectOption(ruleContext);
  await form.locator('button[type="submit"]').click();
  await expect(page.locator('#build-input')).toHaveValue(unsent);
  await page.reload();
  await expect(page.locator('#build-input')).toHaveValue(unsent);
  expect((await savedProject(page)).concepts.find(c => c.name === 'PolicyRevision')?.definition).toBe(older);
});

test('editing an assertion keeps its aggregate owner and never replaces another owners same-key assertion', async ({page}) => {
  const model = structuredClone(initial);
  const a = model.concepts.find(c => c.id === ruleRoot)!;
  a.assertions = [{key:'reviewed',on:'Review',statement:'Original rule contract.'}];
  model.concepts.push({id:'review-root',name:'Review',definition:'A review of proposed changes.',kind:'aggregate',identity:'ReviewID',contextId:ruleContext,invariants:[],assertions:[{key:'reviewed',on:'Accept',statement:'Original review contract.'}],position:[4,1,4]});
  await openRealModel(page,model); await openRule(page);
  await page.locator('[data-edit-assertion="reviewed"]').click();
  const form = page.locator('#dialog-form');
  await expect(form.locator('[name="ownerId"] option')).toHaveCount(1);
  await expect(form.locator('[name="ownerId"]')).toHaveValue(ruleRoot);
  await form.locator('[name="statement"]').fill('Revised rule contract.');
  await form.locator('button[type="submit"]').click();
  const saved = await savedProject(page);
  expect(saved.concepts.find(c => c.id === ruleRoot)?.assertions?.[0].statement).toBe('Revised rule contract.');
  expect(saved.concepts.find(c => c.id === 'review-root')?.assertions?.[0].statement).toBe('Original review contract.');
});


test('a fresh workspace opens the actual bound project rather than the optional example', async ({page}) => {
  await page.goto('/');
  await expect(page.locator('#project-name')).toHaveText('arclint');
  await expect(page.locator('.study-label[data-subject-id]')).toHaveCount(initial.contexts.length);
  expect(exportDomainYaml(await savedProject(page))).toEqual(exportDomainYaml(initial));
});

test('a late repository response cannot overwrite a newly typed stakeholder draft', async ({page}) => {
  let release!: () => void;
  const waiting = new Promise<void>(resolve => { release = resolve; });
  await page.route('**/api/arclint/project', async route => { await waiting; await route.continue(); });
  await page.goto('/');
  const text = 'An unfinished thought before code exists.';
  await page.locator('#build-input').fill(text);
  const response = page.waitForResponse('**/api/arclint/project');
  release(); await response;
  await expect(page.locator('#build-input')).toHaveValue(text);
  await expect(page.locator('#project-name')).toHaveText('Untitled domain');
  expect((await savedProject(page)).contexts).toHaveLength(0);
});
