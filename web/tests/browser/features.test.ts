/*
 * Feature writing over the real embedded Studio and a real `arclint map
 * serve` for the language. The feature store is a STUB answered at the
 * network layer by tests/support/feature-store-stub.ts, because this
 * checkout's Go server has no api/feature routes yet; the parent runs the
 * same flows against the real routes once integrated. Titles say [stub
 * store] where the stub answers.
 */
import { expect, test, type BrowserContext, type Page } from "@playwright/test";
import { mkdirSync } from "node:fs";
import path from "node:path";
import { FeatureStoreStub } from "../support/feature-store-stub";
import { StudioPage } from "../support/studio-page";

const SHOTS = path.join(process.cwd(), "test-results", "screenshots");

async function stubStore(context: BrowserContext, stub: FeatureStoreStub): Promise<void> {
  await context.route(/\/api\/feature(s)?(\?.*)?$/, async (route) => {
    const request = route.request();
    const answer = stub.answer(request.method(), new URL(request.url()), request.postData(), await request.headerValue("if-match"));
    if (!answer) {
      await route.fallback();
      return;
    }
    await route.fulfill({ status: answer.status, contentType: "application/json", body: JSON.stringify(answer.body) });
  });
}

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
  test("[stub store] write, inspect a cue, tie and mark plain, save, reopen in a new session, edit the language, return, copy as ticket", async ({ browser }) => {
    const stub = new FeatureStoreStub();
    const first = await browser.newContext({ viewport: { width: 1440, height: 900 } });
    await first.grantPermissions(["clipboard-read", "clipboard-write"]);
    await stubStore(first, stub);
    const page = await first.newPage();
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

    // Selecting a phrase offers the language to tie it to, with each candidate's context.
    await editor(page).dblclick({ position: { x: 10, y: 10 } });
    await page.keyboard.press("End");
    await page.keyboard.press("Shift+Home");
    const selection = page.getByRole("region", { name: "Selection" });
    await expect(selection).toBeVisible();
    await selection.getByRole("searchbox").fill("password");
    await expect(selection.getByRole("option").first()).toContainText("Password");
    await shoot(page, "feature-02-selection-tie");
    await page.keyboard.press("Escape");

    // Save through the store; the saved document carries UTF-16 positions and the quotes.
    await page.getByRole("button", { name: "Save", exact: true }).click();
    await expect(page.getByRole("status")).toContainText("Saved feature “Invite a person into a tenant”");
    await expect(page.getByRole("navigation", { name: "Features" })).toContainText("Invite a person into a tenant");
    expect(stub.files.size).toBe(1);
    const saved = [...stub.files.values()][0];
    expect(saved.body).toBe("A Tenant invites a Person by their EmailAddress. The tenant owner sets a password for them.");
    expect((saved.references as { quote: string; from: number; to: number; reading: string; term?: { name: string } }[]).map((r) => [r.quote, r.from, r.to, r.reading, r.term?.name ?? null])).toEqual([
      ["Tenant", 2, 8, "names", "Tenant"],
      ["tenant", 53, 59, "plain", null],
    ]);
    await shoot(page, "feature-03-saved");

    // A new browser session: the saved draft reopens from the store, with its marks.
    const second = await browser.newContext({ viewport: { width: 1440, height: 900 } });
    await second.grantPermissions(["clipboard-read", "clipboard-write"]);
    await stubStore(second, stub);
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

  test("[stub store] a stale save writes nothing, keeps the text and offers to write over or take the stored text", async ({ browser }) => {
    const stub = new FeatureStoreStub();
    stub.save({ title: "Refund a ticket", body: "A Tenant refunds.", references: [] });
    const id = [...stub.files.keys()][0];
    const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
    await stubStore(context, stub);
    const page = await context.newPage();
    await openFeatures(page);
    await page.getByRole("navigation", { name: "Features" }).getByRole("button", { name: "Refund a ticket" }).click();
    await expect(editor(page)).toContainText("A Tenant refunds.");

    await editor(page).click();
    await page.keyboard.press("End");
    await page.keyboard.type(" The Person is told.");
    await expect(page.getByRole("navigation", { name: "Features" })).toContainText("unsaved");

    stub.edit(id, "Refund a ticket (renamed elsewhere)");
    await page.getByRole("button", { name: "Save", exact: true }).click();
    const alert = page.getByRole("alert");
    await expect(alert).toContainText("changed in the store since you opened it");
    await expect(alert).toContainText("Refund a ticket (renamed elsewhere)");
    await expect(editor(page)).toContainText("The Person is told.");
    expect(stub.files.get(id)!.body).toBe("A Tenant refunds.");
    await shoot(page, "feature-06-stale-save");

    await page.getByRole("button", { name: "Save mine over it" }).click();
    await expect(page.getByRole("status")).toContainText("Saved feature");
    expect(stub.files.get(id)!.body).toBe("A Tenant refunds. The Person is told.");
    expect(stub.files.get(id)!.title).toBe("Refund a ticket");
    await context.close();
  });

  test("[stub store] a deleted target stays an unresolved annotation with its reason, and deleting a feature removes it from the store", async ({ browser }) => {
    const stub = new FeatureStoreStub();
    const body = "A Gadget is shipped.";
    stub.save({ title: "Ship a gadget", body, references: [{ id: "x", from: 2, to: 8, quote: "Gadget", reading: "names", term: { concept: "value_object", context: "identityaccess", owner: "", name: "Gadget" } }] });
    const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
    await stubStore(context, stub);
    const page = await context.newPage();
    await openFeatures(page);
    await page.getByRole("navigation", { name: "Features" }).getByRole("button", { name: "Ship a gadget" }).click();
    await expect(page.locator(".st-use--unresolved")).toHaveText("Gadget");
    const unresolved = page.getByRole("region", { name: "Unresolved annotations" });
    await expect(unresolved).toContainText("Gadget (value object in identityaccess) is not in the recording now.");
    await shoot(page, "feature-07-unresolved");

    await page.getByRole("button", { name: "Copy as ticket" }).click();
    // No clipboard permission in this context: the ticket is shown for copying by hand, and it names the unresolved use without asserting intent.
    const fallback = page.getByRole("group", { name: "Ticket text" });
    await expect(fallback).toBeVisible();
    await expect(fallback.getByRole("textbox", { name: "Ticket Markdown" })).toHaveValue(/### Unresolved\n\n- “Gadget” tied to Gadget \(value object in identityaccess\): Gadget \(value object in identityaccess\) is not in the recording now\./);
    await fallback.getByRole("button", { name: "Close" }).click();

    await page.getByRole("button", { name: "Delete…" }).click();
    await page.getByRole("group", { name: "Confirm deletion" }).getByRole("button", { name: "Delete", exact: true }).click();
    await expect(page.getByRole("status")).toContainText("Deleted the feature.");
    expect(stub.files.size).toBe(0);
    await expect(page.getByRole("navigation", { name: "Features" })).toContainText("No feature is saved yet");
    await context.close();
  });

  test("a feature draft survives switching to Language and Map and a reload, and keeps every request under a nested base", async ({ page }) => {
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
    // Without the stub, this checkout's server has no feature routes: the list says so without hiding the view.
    expect(api).toContain("/nested/studio/api/features");
    await expect(page.getByRole("navigation", { name: "Features" }).getByRole("alert")).toContainText("404");
  });
});
