/* Browser flows over the Go binary's embedded Studio and real project files. */
import { expect, test, type APIRequestContext, type Page } from "@playwright/test";
import { mkdirSync, readFileSync, realpathSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import type { FeatureDocument, SaveFeatureRequest } from "../../src/shared/transport/index";
import { StudioPage } from "../support/studio-page";

const SHOTS = path.join(process.cwd(), "test-results", "screenshots");
const PROJECT = path.join(realpathSync(tmpdir()), `arclint-studio-e2e-${process.env.STUDIO_E2E_PORT ?? 4421}`, "projects", "identity");

async function listFeatures(request: APIRequestContext): Promise<FeatureDocument[]> {
  const result = await request.get("/api/features");
  expect(result.status()).toBe(200);
  const documents: FeatureDocument[] = [];
  for (const summary of (await result.json()).features) {
    const response = await request.get(`/api/feature?id=${summary.id}`);
    expect(response.status()).toBe(200);
    documents.push((await response.json()).feature);
  }
  return documents;
}

async function saveFeature(request: APIRequestContext, data: SaveFeatureRequest): Promise<FeatureDocument> {
  const response = await request.post("/api/feature", { data });
  expect(response.status()).toBe(200);
  return (await response.json()).feature;
}

test.beforeEach(async ({ request }) => {
  for (const feature of await listFeatures(request)) {
    const response = await request.delete(`/api/feature?id=${feature.id}`, { headers: { "If-Match": feature.revision } });
    expect(response.status()).toBe(200);
  }
});

async function openFeatures(page: Page, base = "/"): Promise<StudioPage> {
  const studio = new StudioPage(page, base);
  await studio.open();
  await page.getByRole("tab", { name: "Features" }).click();
  await expect(page.getByRole("navigation", { name: "Features" })).toBeVisible();
  return studio;
}

function editor(page: Page) {
  return page.locator(".st-editor-host .cm-content");
}

async function shoot(page: Page, name: string): Promise<void> {
  mkdirSync(SHOTS, { recursive: true });
  await page.screenshot({ path: path.join(SHOTS, `${name}.png`), fullPage: false });
}

test.describe("writing a feature with the language at hand", () => {
  test("write, inspect a cue, tie and mark plain, save, reopen in a new session, edit the language, return, copy as ticket", async ({ browser, request }) => {
    const first = await browser.newContext({ viewport: { width: 1440, height: 900 } });
    await first.grantPermissions(["clipboard-read", "clipboard-write"]);
    const page = await first.newPage();
    const domainBefore = readFileSync(path.join(PROJECT, "domain.arclint.yaml"), "utf8");
    await openFeatures(page);
    await expect(page.getByRole("navigation", { name: "Features" })).toContainText("No feature is saved yet");

    await page.getByRole("button", { name: "+ Feature" }).first().click();
    await page.getByRole("textbox", { name: "Title" }).fill("Invite a person into a tenant");
    await editor(page).click();
    await page.keyboard.type("A Tenant invites a Person by their EmailAddress. The tenant owner sets a password for them.");

    // Automatic cues: recorded names light up; "tenant" and "password" in lower case are the same spellings, so they cue too.
    const cues = page.locator(".st-use--cue");
    await expect(cues).toHaveCount(5);
    await expect(cues.nth(0)).toHaveText("Tenant");
    await expect(cues.nth(1)).toHaveText("Person");
    await expect(cues.nth(2)).toHaveText("EmailAddress");
    await expect(cues.nth(3)).toHaveText("tenant");
    await expect(cues.nth(4)).toHaveText("password");

    // The inspector shows the definition of the use at the cursor, as recorded.
    await cues.nth(0).click();
    const atCursor = page.getByRole("region", { name: "At the cursor" });
    await expect(atCursor).toContainText("Recognised, not confirmed");
    await expect(atCursor).toContainText("Tenant");
    await expect(atCursor).toContainText("A subscribing organisation that owns users and roles");
    await expect(atCursor.getByRole("region", { name: "Connections" })).toContainText("TenantID");
    await shoot(page, "feature-01-cue-inspected");

    // Deliberate association: tie the first Tenant; mark the lower-case "tenant" plain.
    await atCursor.getByRole("button", { name: "Tie it", exact: true }).click();
    await expect(page.locator(".st-use--tied")).toHaveCount(1);
    await expect(page.locator(".st-use--tied")).toHaveText("Tenant");
    await expect(page.locator(".st-use-mark .st-kind-badge")).toHaveAttribute("aria-label", "Aggregate Tenant");
    await expect(atCursor).toContainText("Tied on purpose");

    await page.locator(".st-use--cue", { hasText: "tenant" }).click();
    await atCursor.getByRole("button", { name: "Mark plain" }).click();
    await expect(page.locator(".st-use--plain")).toHaveText("tenant");
    await expect(page.locator(".st-use--cue")).toHaveCount(3);
    await expect(atCursor).toContainText("Plain word");

    // Save through the store; the saved document carries UTF-16 positions and the quotes.
    await page.getByRole("button", { name: "Save", exact: true }).click();
    await expect(page.getByRole("status")).toContainText("Saved feature “Invite a person into a tenant”");
    await expect(page.getByRole("button", { name: "Save", exact: true })).toBeDisabled();
    await expect(page.getByRole("button", { name: "Discard changes" })).toHaveCount(0);
    await expect(page.getByRole("navigation", { name: "Features" })).toContainText("Invite a person into a tenant");
    const allSaved = await listFeatures(request);
    expect(allSaved).toHaveLength(1);
    const saved = allSaved[0];
    const file = JSON.parse(readFileSync(path.join(PROJECT, ".arclint/studio/features", `${saved.id}.json`), "utf8"));
    expect(file.version).toBe(1);
    expect(file.feature.body).toBe(saved.body);
    expect(file.feature.references).toEqual(saved.references);
    expect(readFileSync(path.join(PROJECT, "domain.arclint.yaml"), "utf8")).toBe(domainBefore);
    expect(saved.body).toBe("A Tenant invites a Person by their EmailAddress. The tenant owner sets a password for them.");
    expect((saved.references as { quote: string; from: number; to: number; reading: string; term?: { name: string } }[]).map((r) => [r.quote, r.from, r.to, r.reading, r.term?.name ?? null])).toEqual([
      ["Tenant", 2, 8, "names", "Tenant"],
      ["tenant", 53, 59, "plain", null],
    ]);
    await shoot(page, "feature-03-saved");

    // A new browser session: the saved draft reopens from the store, with its marks.
    const second = await browser.newContext({ viewport: { width: 1440, height: 900 } });
    await second.grantPermissions(["clipboard-read", "clipboard-write"]);
    const again = await second.newPage();
    await openFeatures(again);
    await again.getByRole("navigation", { name: "Features" }).getByRole("button", { name: "Invite a person into a tenant" }).click();
    await expect(again.getByRole("textbox", { name: "Title" })).toHaveValue("Invite a person into a tenant");
    // The badge drawn left of a tied use is a widget: it reads "A" in the text content, never in the saved body.
    await expect(editor(again)).toContainText("Tenant invites a Person by their EmailAddress");
    await expect(again.locator(".st-use--tied")).toHaveText("Tenant");
    await expect(again.locator(".st-use--plain")).toHaveText("tenant");
    await expect(again.locator(".st-use--cue")).toHaveCount(3);
    await shoot(again, "feature-04-reopened-new-session");

    // Edit the language from the feature (real Go write), come back, and the text and the new definition are both there.
    await again.locator(".st-use--cue", { hasText: "EmailAddress" }).click();
    await again.getByRole("region", { name: "At the cursor" }).getByRole("button", { name: "Open in Language" }).click();
    const studio = new StudioPage(again, "/");
    await expect(studio.card.getByRole("heading", { level: 1 })).toHaveText("EmailAddress");
    await studio.card.getByRole("button", { name: "Edit", exact: true }).click();
    await studio.field("Definition").fill("A well-formed email address, verified before an invitation is sent.");
    await studio.card.getByRole("button", { name: "Save" }).click();
    await expect(again.getByRole("status")).toContainText(/updated value object EmailAddress/);
    await again.getByRole("tab", { name: "Features" }).click();
    await expect(editor(again)).toContainText("Tenant invites a Person by their EmailAddress");
    await again.locator(".st-use--cue", { hasText: "EmailAddress" }).click();
    await expect(again.getByRole("region", { name: "At the cursor" })).toContainText("verified before an invitation is sent");
    await shoot(again, "feature-05-returned-with-new-definition");

    // Copy as ticket: deliberate ties apart from cues, current definitions, no plain word listed.
    await again.getByRole("button", { name: "Copy as ticket" }).click();
    await expect(again.getByRole("status").filter({ hasText: "on the clipboard" })).toBeVisible();
    const ticket = await again.evaluate(() => navigator.clipboard.readText());
    expect(ticket).toContain("# Invite a person into a tenant");
    expect(ticket).toContain("### Tied on purpose\n\n- **Tenant** (aggregate in identityaccess): A subscribing organisation that owns users and roles");
    expect(ticket).toContain("### Recognised automatically");
    expect(ticket).toContain("- **EmailAddress** (value object in identityaccess): A well-formed email address, verified before an invitation is sent.");
    expect(ticket).toContain("- **Person** (value object in identityaccess)");
    expect(ticket).not.toContain("plain");
    expect(ticket.split("**Tenant**").length).toBe(2);
    await second.close();
    await first.close();
  });

  test("a stale save writes nothing, keeps the text and offers to write over or take the stored text", async ({ browser, request }) => {
    const stored = await saveFeature(request, { title: "Refund a ticket", body: "A Tenant refunds.", references: [] });
    const id = stored.id;
    const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
    const page = await context.newPage();
    await openFeatures(page);
    await page.getByRole("navigation", { name: "Features" }).getByRole("button", { name: "Refund a ticket" }).click();
    await expect(editor(page)).toContainText("A Tenant refunds.");

    await editor(page).click();
    await page.keyboard.press("End");
    await page.keyboard.type(" The Person is told.");
    await expect(page.getByRole("navigation", { name: "Features" })).toContainText("unsaved");

    await saveFeature(request, { ...stored, title: "Refund a ticket (renamed elsewhere)" });
    await page.getByRole("button", { name: "Save", exact: true }).click();
    const alert = page.getByRole("alert");
    await expect(alert).toContainText("changed in the store since you opened it");
    await expect(alert).toContainText("Refund a ticket (renamed elsewhere)");
    await expect(editor(page)).toContainText("The Person is told.");
    expect((await listFeatures(request)).find((f) => f.id === id)!.body).toBe("A Tenant refunds.");
    await shoot(page, "feature-06-stale-save");

    await page.getByRole("button", { name: "Save mine over it" }).click();
    await expect(page.getByRole("status")).toContainText("Saved feature");
    expect((await listFeatures(request)).find((f) => f.id === id)!.body).toBe("A Tenant refunds. The Person is told.");
    expect((await listFeatures(request)).find((f) => f.id === id)!.title).toBe("Refund a ticket");
    await context.close();
  });

  test("a deleted target stays an unresolved annotation with its reason, and deleting a feature removes it from the store", async ({ browser, request }) => {
    const body = "A Gadget is shipped.";
    await saveFeature(request, { title: "Ship a gadget", body, references: [{ id: "x", from: 2, to: 8, quote: "Gadget", reading: "names", term: { concept: "value_object", context: "identityaccess", owner: "", name: "Gadget" } }, { id: "outside", from: 900, to: 905, quote: "Tenant", reading: "names", term: { concept: "aggregate", context: "identityaccess", owner: "", name: "Tenant" } }] });
    const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
    const page = await context.newPage();
    await openFeatures(page);
    await page.getByRole("navigation", { name: "Features" }).getByRole("button", { name: "Ship a gadget" }).click();
    await expect(page.locator(".st-use--unresolved")).toHaveText("Gadget");
    const unresolved = page.getByRole("region", { name: "Unresolved annotations" });
    await expect(unresolved).toContainText("Gadget (value object in identityaccess) is not in the recording now.");
    await shoot(page, "feature-07-unresolved");
    await expect(unresolved).toContainText("Its position lies outside the text.");
    await editor(page).click();
    await page.keyboard.press("ControlOrMeta+End");
    await page.keyboard.type(" Writing continues.");
    await page.getByRole("button", { name: "Save", exact: true }).click();
    await expect(page.getByRole("status")).toContainText("Saved feature");
    const preserved = (await listFeatures(request))[0];
    expect(preserved.body).toBe(body + " Writing continues.");
    expect(preserved.references.find((r) => r.id === "outside")).toMatchObject({ from: 900, to: 905, quote: "Tenant" });

    await page.getByRole("button", { name: "Copy as ticket" }).click();
    // No clipboard permission in this context: the ticket is shown for copying by hand, and it names the unresolved use without asserting intent.
    const fallback = page.getByRole("group", { name: "Ticket text" });
    await expect(fallback).toBeVisible();
    await expect(fallback.getByRole("textbox", { name: "Ticket Markdown" })).toHaveValue(/### Unresolved\n\n- “Gadget” tied to Gadget \(value object in identityaccess\): Gadget \(value object in identityaccess\) is not in the recording now\./);
    await fallback.getByRole("button", { name: "Close" }).click();

    await page.getByRole("button", { name: "Delete…" }).click();
    await page.getByRole("group", { name: "Confirm deletion" }).getByRole("button", { name: "Delete", exact: true }).click();
    await expect(page.getByRole("status")).toContainText("Deleted the feature.");
    expect(await listFeatures(request)).toHaveLength(0);
    await expect(page.getByRole("navigation", { name: "Features" })).toContainText("No feature is saved yet");
    await context.close();
  });

  test("a feature draft survives switching to Language and Map and a reload, and keeps every request under a nested base", async ({ page, request }) => {
    const api: string[] = [];
    page.on("request", (r) => {
      if (r.url().includes("/api/")) api.push(new URL(r.url()).pathname);
    });
    await openFeatures(page, "/nested/studio/");
    await page.getByRole("button", { name: "+ Feature" }).first().click();
    await page.getByRole("textbox", { name: "Title" }).fill("Kept across modes");
    await editor(page).click();
    await page.keyboard.type("A Tenant waits.");
    await page.getByRole("tab", { name: "Language" }).click();
    await expect(page.getByRole("navigation", { name: "The recorded language" })).toBeVisible();
    await page.getByRole("tab", { name: "Map" }).click();
    await page.getByRole("tab", { name: "Features" }).click();
    await expect(page.getByRole("textbox", { name: "Title" })).toHaveValue("Kept across modes");
    await expect(editor(page)).toContainText("A Tenant waits.");
    await page.reload();
    await expect(page.getByRole("textbox", { name: "Title" })).toHaveValue("Kept across modes");
    await expect(editor(page)).toContainText("A Tenant waits.");
    await expect(page.getByRole("navigation", { name: "Features" })).toContainText("Not saved yet");
    for (const p of api) expect(p.startsWith("/nested/studio/api/")).toBe(true);
    expect(api).toContain("/nested/studio/api/features");
    await expect(page.getByRole("navigation", { name: "Features" }).getByRole("alert")).toHaveCount(0);
    expect(await listFeatures(request)).toHaveLength(0);
  });
});


test("typing during a pending save remains unsaved until the next save", async ({ page, request }) => {
  await openFeatures(page);
  await page.getByRole("button", { name: "+ Feature" }).first().click();
  await page.getByRole("textbox", { name: "Title" }).fill("In flight");
  await editor(page).click();
  await page.keyboard.type("A Tenant waits.");
  let release!: () => void;
  let reached!: () => void;
  const hold = new Promise<void>((done) => { release = done; });
  const committed = new Promise<void>((done) => { reached = done; });
  let first = true;
  await page.route("**/api/feature", async (route) => {
    if (!first || route.request().method() !== "POST") return route.continue();
    first = false;
    const response = await route.fetch();
    reached();
    await hold;
    await route.fulfill({ response });
  });
  await page.getByRole("button", { name: "Save", exact: true }).click();
  await committed;
  await page.getByRole("textbox", { name: "Title" }).fill("Newer typing");
  await editor(page).click();
  await page.keyboard.press("ControlOrMeta+End");
  await page.keyboard.type(" The Person arrives.");
  release();
  await expect(page.getByRole("status")).toContainText("Saved feature “In flight”");
  await expect(editor(page)).toContainText("The Person arrives.");
  await expect(page.getByRole("textbox", { name: "Title" })).toHaveValue("Newer typing");
  await expect(page.getByRole("navigation", { name: "Features" })).toContainText("unsaved");
  expect((await listFeatures(request))[0].body).toBe("A Tenant waits.");
  await editor(page).click();
  await page.keyboard.press("ControlOrMeta+End");
  await page.keyboard.type(" More detail.");
  await page.getByRole("button", { name: "Save", exact: true }).click();
  await expect(page.getByRole("status")).toContainText("Saved feature “Newer typing”");
  expect((await listFeatures(request))[0].body).toBe("A Tenant waits. The Person arrives. More detail.");
  await editor(page).click();
  await page.keyboard.press("ControlOrMeta+z");
  await expect(editor(page)).toContainText("The Person arrives.");
  await expect(editor(page)).not.toContainText("More detail.");
});

test("a recorded spelling in two contexts stays ambiguous until the author chooses", async ({ page, request }) => {
  for (const data of [
    { concept: "bounded_context", name: "support", change: { definition: "Helping customers use their accounts." } },
    { concept: "value_object", context: "support", name: "Tenant", change: { definition: "The support team's record of a subscribing organisation." } },
  ]) {
    const response = await request.post("/api/define", { data });
    expect(response.status()).toBe(200);
  }
  await openFeatures(page);
  await page.getByRole("button", { name: "+ Feature" }).first().click();
  await editor(page).click();
  await page.keyboard.type("A Tenant asks for help.");
  await expect(page.locator(".st-use--ambiguous")).toHaveText("Tenant");
  await page.locator(".st-use--ambiguous").click();
  const cursor = page.getByRole("region", { name: "At the cursor" });
  await expect(cursor).toContainText("identityaccess");
  await expect(cursor).toContainText("support");
  await cursor.getByRole("listitem").filter({ hasText: "support" }).getByRole("button", { name: "Tie to this" }).click();
  await expect(page.locator(".st-use--tied")).toHaveText("Tenant");
  await page.getByRole("button", { name: "Save", exact: true }).click();
  await expect(page.getByRole("status")).toContainText("Saved feature");
  expect((await listFeatures(request))[0].references[0].term!.context).toBe("support");
});

for (const operation of ["save", "delete"] as const) {
  test(`a delayed ${operation} conflict stays with its originating feature`, async ({ page, request }) => {
    const alpha = await saveFeature(request, { title: "Alpha", body: "A Tenant waits.", references: [] });
    const beta = await saveFeature(request, { title: "Beta", body: "A Person arrives.", references: [] });
    await openFeatures(page);
    const features = page.getByRole("navigation", { name: "Features" });
    await features.getByRole("button", { name: "Alpha", exact: true }).click();
    await expect(editor(page)).toContainText(alpha.body);
    await editor(page).click();
    await page.keyboard.press("ControlOrMeta+End");
    await page.keyboard.type(" Alpha's unsaved addition.");
    await saveFeature(request, { ...alpha, title: "Alpha changed elsewhere" });
    let release!: () => void;
    let reached!: () => void;
    const hold = new Promise<void>((done) => { release = done; });
    const started = new Promise<void>((done) => { reached = done; });
    await page.route(/\/api\/feature(?:\?.*)?$/, async (route) => {
      if (route.request().method() !== (operation === "save" ? "POST" : "DELETE")) return route.continue();
      const response = await route.fetch();
      reached();
      await hold;
      await route.fulfill({ response });
    });
    if (operation === "save") await page.getByRole("button", { name: "Save", exact: true }).click();
    else {
      await page.getByRole("button", { name: "Delete…" }).click();
      await page.getByRole("group", { name: "Confirm deletion" }).getByRole("button", { name: "Delete", exact: true }).click();
    }
    await started;
    await features.getByRole("button", { name: "Beta", exact: true }).click();
    await expect(editor(page)).toContainText(beta.body);
    await editor(page).click();
    await page.keyboard.press("ControlOrMeta+End");
    await page.keyboard.type(" Beta's unsaved addition.");
    release();
    await expect(page.getByRole("button", { name: "Save", exact: true })).toBeEnabled();
    await expect(page.getByRole("alert")).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Take the stored text" })).toHaveCount(0);
    await expect(editor(page)).toContainText("Beta's unsaved addition.");
    await features.getByRole("button", { name: /Alpha/ }).click();
    await expect(page.getByRole("alert").first()).toContainText("Alpha changed elsewhere");
    await expect(editor(page)).toContainText("Alpha's unsaved addition.");
    const documents = await listFeatures(request);
    expect(documents.find((doc) => doc.id === beta.id)!.body).toBe(beta.body);
    expect(documents.find((doc) => doc.id === alpha.id)!.body).toBe(alpha.body);
  });
}

test("deletion confirmation is reset when another feature is selected", async ({ page, request }) => {
  const alpha = await saveFeature(request, { title: "Delete Alpha", body: "Alpha", references: [] });
  const beta = await saveFeature(request, { title: "Delete Beta", body: "Beta", references: [] });
  await openFeatures(page);
  const features = page.getByRole("navigation", { name: "Features" });
  await features.getByRole("button", { name: "Delete Beta", exact: true }).click();
  await expect(editor(page)).toContainText(beta.body);
  await features.getByRole("button", { name: "Delete Alpha", exact: true }).click();
  await expect(editor(page)).toContainText(alpha.body);
  await page.getByRole("button", { name: "Delete…" }).click();
  await expect(page.getByRole("group", { name: "Confirm deletion" })).toBeVisible();
  await features.getByRole("button", { name: "Delete Beta", exact: true }).click();
  await expect(editor(page)).toContainText(beta.body);
  await expect(page.getByRole("group", { name: "Confirm deletion" })).toHaveCount(0);
  await page.getByRole("button", { name: "Delete…" }).click();
  await page.getByRole("group", { name: "Confirm deletion" }).getByRole("button", { name: "Delete", exact: true }).click();
  await expect(page.getByRole("status")).toContainText("Deleted the feature.");
  expect((await listFeatures(request)).map((doc) => doc.id)).toEqual([alpha.id]);
});

for (const destination of ["search result", "project name"] as const) {
  test(`opens Language from Features using the global ${destination}, keeping the feature draft`, async ({ page, request }) => {
    const studio = await openFeatures(page);
    const project = (await studio.overview()).project!;
    await page.getByRole("button", { name: "+ Feature" }).first().click();
    await page.getByRole("textbox", { name: "Title" }).fill("Keep my draft");
    await editor(page).click();
    await page.keyboard.type("A Tenant waits.");

    if (destination === "search result") {
      await studio.search.fill("EmailAddress");
      await page.getByRole("option", { name: /^Value object EmailAddress\b/ }).click();
    } else await page.getByTitle("The project and its description", { exact: true }).click();
    await expect(page.getByRole("tab", { name: "Language", exact: true })).toHaveAttribute("aria-selected", "true");
    await expect(studio.card.getByRole("heading", { level: 1 })).toHaveText(destination === "search result" ? "EmailAddress" : project);

    await page.getByRole("tab", { name: "Features", exact: true }).click();
    await expect(page.getByRole("textbox", { name: "Title" })).toHaveValue("Keep my draft");
    await expect(editor(page)).toContainText("A Tenant waits.");
    expect(await listFeatures(request)).toEqual([]);

    await page.getByRole("tab", { name: "Map", exact: true }).click();
    if (destination === "search result") {
      await studio.search.fill("Person");
      await page.getByRole("option", { name: /^Value object Person\b/ }).click();
    } else await page.getByTitle("The project and its description", { exact: true }).click();
    await expect(page.getByRole("tab", { name: "Map", exact: true })).toHaveAttribute("aria-selected", "true");
    await expect(page.getByRole("complementary", { name: "Inspector" }).getByRole("heading", { level: 1 })).toHaveText(destination === "search result" ? "Person" : project);
  });
}

for (const typing of [false, true]) {
  test(`a stale delete synchronizes the stored snapshot ${typing ? "after preserving pending typing until discarded" : "when there is no local draft"}`, async ({ page, request }) => {
    const stored = await saveFeature(request, {
      title: "Snapshot sync", body: "A Tenant waits.",
      references: [{ id: "tenant", from: 2, to: 8, quote: "Tenant", reading: "names", term: { concept: "aggregate", context: "identityaccess", owner: "", name: "Tenant" } }],
    });
    await openFeatures(page);
    await page.getByRole("navigation", { name: "Features" }).getByRole("button", { name: stored.title, exact: true }).click();
    await expect(editor(page)).toContainText("Tenant waits.");
    const changed = await saveFeature(request, {
      ...stored, title: "Changed in another session", body: "A Person arrives.",
      references: [{ id: "person", from: 2, to: 8, quote: "Person", reading: "names", term: { concept: "value_object", context: "identityaccess", owner: "", name: "Person" } }],
    });
    let release!: () => void;
    let reached!: () => void;
    const held = new Promise<void>((done) => { release = done; });
    const started = new Promise<void>((done) => { reached = done; });
    await page.route(/\/api\/feature(?:\?.*)?$/, async (route) => {
      if (route.request().method() !== "DELETE") return route.continue();
      const response = await route.fetch();
      reached();
      await held;
      await route.fulfill({ response });
    });
    await page.getByRole("button", { name: "Delete…" }).click();
    await page.getByRole("group", { name: "Confirm deletion" }).getByRole("button", { name: "Delete", exact: true }).click();
    await started;
    try {
      if (typing) {
        await editor(page).click();
        await page.keyboard.press("ControlOrMeta+End");
        await page.keyboard.type(" Local typing.");
      }
    } finally {
      release();
    }
    await expect(page.getByRole("alert").first()).toContainText("changed in the store");
    if (typing) {
      await expect(editor(page)).toContainText("Tenant waits. Local typing.");
      await expect(page.locator(".st-use--tied")).toHaveText("Tenant");
      await page.getByRole("button", { name: "Take the stored text" }).click();
    }
    await expect(page.getByRole("textbox", { name: "Title" })).toHaveValue(changed.title);
    await expect(editor(page)).toContainText("Person arrives.");
    await expect(editor(page)).not.toContainText("Tenant waits.");
    await expect(page.locator(".st-use--tied")).toHaveText("Person");

    await editor(page).click();
    await page.keyboard.press("ControlOrMeta+End");
    await page.keyboard.type(" More detail.");
    await page.getByRole("button", { name: "Save", exact: true }).click();
    await expect(page.getByRole("status")).toContainText(`Saved feature “${changed.title}”`);
    const saved = (await listFeatures(request))[0];
    expect(saved.body).toBe(changed.body + " More detail.");
    expect(saved.references).toEqual(changed.references);
  });
}


test("selection search filters the recorded language and supports keyboard picking", async ({ page, request }) => {
  const body = "A Tenant invites a Person by their EmailAddress. The tenant owner sets a password for them.";
  const stored = await saveFeature(request, { title: "Search a selection", body, references: [] });
  await openFeatures(page);
  await page.getByRole("navigation", { name: "Features" }).getByRole("button", { name: stored.title, exact: true }).click();
  await expect(editor(page)).toContainText(body);
  await editor(page).click();
  await page.keyboard.press("ControlOrMeta+a");
  const selection = page.getByRole("region", { name: "Selection" });
  await expect(selection.getByText(`“${body}”`, { exact: true })).toBeVisible();
  const query = selection.getByRole("searchbox");
  await query.fill("password");
  await expect(query).toHaveValue("password");
  await expect(selection.getByRole("option").first()).toContainText("Password");
  await selection.getByRole("option").first().focus();
  await page.keyboard.press("End");
  await page.keyboard.press("Home");
  await page.keyboard.press("Enter");
  await expect(selection.getByRole("button", { name: "Tie", exact: true })).toBeEnabled();
  await expect(query).toHaveValue("password");
  expect((await listFeatures(request))[0]).toEqual(stored);
});
