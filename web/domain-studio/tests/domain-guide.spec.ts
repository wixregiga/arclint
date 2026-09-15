import { expect, test, type Page } from '@playwright/test';
import { createEmptyProject, createExampleProject } from '../src/domain';
import { savedProject, toolButton } from './studio.helpers';

async function openGuide(page: Page, text: string, project = createExampleProject()) {
  await page.addInitScript(project => {
    if (!localStorage.getItem('arclint.domain-studio.v2')) localStorage.setItem('arclint.domain-studio.v1', JSON.stringify({project, baseline:null, patterns:[]}));
  }, project);
  await page.goto('/');
  await expect(page.locator('#project-name')).toHaveText(project.name);
  await page.locator('#build-input').fill(text);
  await page.locator('#build-draft').click();
  await page.getByRole('button', {name:'Help me choose'}).click();
  await expect(page.locator('#guide-question')).toHaveText('What are you trying to describe?');
}

const notebook = (page: Page) => page.evaluate(() => JSON.parse(localStorage.getItem('arclint.domain-studio.v2')!).notebook);
const answer = (page: Page, name: RegExp) => page.locator('#modal').getByRole('button', {name}).click();

test('guided answers propose a value object and preserve the original draft through review, save and undo', async ({page}) => {
  const original = createExampleProject();
  const text = '  A revision identifies one published policy.\nIts value remains stable.  ';
  await openGuide(page, text, original);
  await answer(page, /^Something people work with/);
  await answer(page, /^A thing people name/);
  await answer(page, /^Only its values matter/);
  await expect(page.locator('#guide-question')).toHaveText('Value object');
  expect(await savedProject(page)).toEqual(original);
  await answer(page, /^Review fields$/);
  await expect(page.locator('#dialog-form [name="definition"]')).toHaveValue(text);
  await expect(page.locator('#dialog-form [name="kind"]')).toHaveValue('value_object');
  await page.locator('#dialog-form [name="name"]').fill('PolicyRevision');
  await page.locator('#dialog-form button[type="submit"]').click();
  expect((await savedProject(page)).concepts.find(c => c.name === 'PolicyRevision')?.definition).toBe(text.trim());
  expect(await notebook(page)).toHaveLength(0);
  await toolButton(page, 'Undo');
  expect((await savedProject(page)).concepts.some(c => c.name === 'PolicyRevision')).toBe(false);
  expect((await notebook(page))[0].text).toBe(text);
  await expect(page.locator('#build-input')).toHaveValue(text);
});

test('a blank project can start with a guided bounded context without knowing a domain type', async ({page}) => {
  const text = 'The UI context is where the web resources are rendered.';
  await openGuide(page, text, createEmptyProject());
  await answer(page, /^A place where words/);
  await expect(page.locator('#guide-question')).toHaveText('Bounded context');
  await answer(page, /^Review fields$/);
  await expect(page.locator('#dialog-form [name="description"]')).toHaveValue(text);
  await page.locator('#dialog-form [name="name"]').fill('UI');
  await answer(page, /^Create context$/);
  const saved = await savedProject(page);
  expect(saved.contexts).toHaveLength(1);
  expect(saved.contexts[0]).toMatchObject({name:'UI',description:text});
  expect(await notebook(page)).toHaveLength(0);
});

test('back, uncertainty and dismissal preserve the note across reload and allow direct assignment', async ({page}) => {
  const text = '  Not sure who owns this promise.\n Ask the delivery team.  ';
  await openGuide(page, text);
  await answer(page, /^A condition that needs/);
  await answer(page, /^A promise the domain/);
  await page.locator('#guide-back').click();
  await expect(page.locator('#guide-question')).toHaveText('Is this a promise or a decision?');
  await answer(page, /I’m not sure yet/);
  await expect(page.locator('#guide-question')).toHaveText('Open question');
  await page.keyboard.press('Escape');
  await expect(page.locator('#modal')).not.toBeVisible();
  await page.reload();
  expect((await notebook(page))[0].text).toBe(text);
  await page.locator('#open-notebook').click();
  await page.locator('[data-assign-draft]').click();
  await page.locator('#start-domain-guide').click();
  await page.locator('#guide-direct').click();
  await page.locator('[data-create-kind="unclassified"]').click();
  await expect(page.locator('#dialog-form [name="definition"]')).toHaveValue(text);
  await page.locator('#dialog-form [name="name"]').fill('Promise ownership');
  await answer(page, /^Save definition$/);
  expect((await savedProject(page)).concepts.find(c => c.name === 'Promise ownership')?.kind).toBe('unclassified');
});

test('a missing assertion owner is explained and creating it does not consume the assertion draft', async ({page}) => {
  const model = createExampleProject();
  model.concepts = []; model.relationships = [];
  const text = 'After publication, the revision is frozen.';
  await openGuide(page, text, model);
  await answer(page, /^A condition that needs/);
  await answer(page, /^A promise the domain/);
  await answer(page, /^After a particular aggregate operation/);
  await expect(page.locator('#guide-question')).toHaveText('Assertion');
  await expect(page.locator('.guide-hint')).toContainText('needs an aggregate owner');
  await answer(page, /^Define an aggregate first$/);
  await expect(page.locator('#dialog-form [name="definition"]')).toHaveValue('');
  await page.locator('#dialog-form [name="name"]').fill('Policy');
  await answer(page, /^Save definition$/);
  expect((await notebook(page))[0].text).toBe(text);
  await expect(page.locator('#build-input')).toHaveValue(text);
  // The same note can now be assigned to the new owner's operation.
  await page.locator('#clear-selection').click();
  await page.locator('#open-notebook').click();
  await page.locator('[data-assign-draft]').click();
  await page.locator('#start-domain-guide').click();
  await answer(page, /^A condition that needs/);
  await answer(page, /^A promise the domain/);
  await answer(page, /^After a particular aggregate operation/);
  await answer(page, /^Review fields$/);
  await expect(page.locator('#dialog-form [name="statement"]')).toHaveValue(text);
  await page.locator('#dialog-form [name="key"]').fill('revision-frozen');
  await page.locator('#dialog-form [name="on"]').fill('Publish');
  await answer(page, /^Save assertion$/);
  expect((await savedProject(page)).concepts.find(c => c.name === 'Policy')?.assertions).toContainEqual({key:'revision-frozen',on:'Publish',statement:text});
  expect(await notebook(page)).toHaveLength(0);
});

test('the guide is keyboard accessible and fits a narrow screen', async ({page}, testInfo) => {
  await page.setViewportSize({width:390,height:844});
  await openGuide(page, 'The UI context is where the web resources are rendered.');
  await expect(page.locator('#guide-question')).toBeFocused();
  await page.keyboard.press('Tab');
  await expect(page.locator('[data-guide-next="bounded_context"]')).toBeFocused();
  const overflow = await page.locator('#modal').evaluate(el => el.scrollWidth > el.clientWidth);
  expect(overflow).toBe(false);
  const back = await page.locator('#guide-back').boundingBox();
  expect(back!.y + back!.height).toBeLessThanOrEqual(844);
  await page.screenshot({path:testInfo.outputPath('guide-mobile.png')});
  await page.keyboard.press('Enter');
  await expect(page.locator('#guide-question')).toHaveText('Bounded context');
  await page.setViewportSize({width:1601,height:778});
  await page.screenshot({path:testInfo.outputPath('guide-desktop.png')});
});
