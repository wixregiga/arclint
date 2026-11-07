/*
 * Screenshots for the pull request, taken through the real host over
 * real recordings. The same steps run against main's binary and this
 * checkout's, so each "before" has its "after" at the same state.
 *
 *   EVIDENCE_DIR=<dir> ARCLINT_BIN=<binary> npx playwright test -c playwright.evidence.config.ts
 */
import { mkdirSync } from "node:fs";
import path from "node:path";
import { expect, test, type Page } from "@playwright/test";
import { StudioPage } from "../support/studio-page";

const OUT = process.env.EVIDENCE_DIR ?? "";
/** "after" also assesses the code and opens a source peek; main's Studio has neither. */
const AFTER = process.env.EVIDENCE_STAGE === "after";

test.skip(OUT === "", "EVIDENCE_DIR names where the screenshots go");

async function shot(page: Page, name: string): Promise<void> {
  mkdirSync(OUT, { recursive: true });
  await page.screenshot({ path: path.join(OUT, `${name}.png`) });
}

/** After: asks for the code evidence in the inspector and waits for the assessment. */
async function assess(page: Page): Promise<void> {
  if (!AFTER) return;
  const inspector = page.getByRole("complementary", { name: "Inspector" });
  const evidence = inspector.getByRole("region", { name: "Code evidence" });
  await evidence.getByRole("button", { name: "Assess code" }).click();
  await expect(evidence.getByText(/^Assessed /)).toBeVisible({ timeout: 60_000 });
}

async function openMap(page: Page, base: string): Promise<StudioPage> {
  const studio = new StudioPage(page, base);
  await studio.open();
  await page.getByRole("tab", { name: "Map" }).click();
  return studio;
}

test("identity: an aggregate selected on the map", async ({ page }) => {
  await openMap(page, "/");
  await page.getByRole("group", { name: "Entries of identityaccess" }).getByRole("button", { name: "Aggregate Tenant" }).click();
  await expect(page.getByRole("complementary", { name: "Inspector" }).getByRole("heading", { level: 1 })).toHaveText("Tenant");
  await assess(page);
  await shot(page, "desktop-identity-aggregate");
  if (!AFTER) return;
  const evidence = page.getByRole("region", { name: "Code evidence" });
  await evidence.getByRole("button", { name: /^Peek at identityaccess\/tenant\/tenant\.go:\d+$/ }).first().click();
  await expect(page.getByRole("region", { name: /^Source peek: / }).locator(".st-peek__line.is-target")).toBeVisible();
  await shot(page, "desktop-identity-aggregate-peek");
  await evidence.scrollIntoViewIfNeeded();
  await page.getByRole("complementary", { name: "Inspector" }).evaluate((el) => el.scrollTo(0, el.scrollHeight));
  await shot(page, "desktop-identity-aggregate-peek-scrolled");
});

test("identity: the bounded context selected on the map", async ({ page }) => {
  const studio = await openMap(page, "/");
  await studio.select("identityaccess");
  await expect(page.getByRole("complementary", { name: "Inspector" }).getByRole("heading", { level: 1 })).toHaveText("identityaccess");
  await assess(page);
  if (AFTER) await page.getByRole("complementary", { name: "Inspector" }).evaluate((el) => el.scrollTo(0, el.scrollHeight));
  await shot(page, "desktop-identity-context");
});

test("boxoffice: an aggregate selected on the map", async ({ page }) => {
  await openMap(page, "/boxoffice/");
  await page.getByRole("group", { name: "Bounded contexts and their relations" }).getByRole("button", { name: "Context ordering" }).click();
  await page.getByRole("group", { name: "Entries of ordering" }).getByRole("button", { name: "Aggregate Order" }).click();
  await expect(page.getByRole("complementary", { name: "Inspector" }).getByRole("heading", { level: 1 })).toHaveText("Order");
  await assess(page);
  await shot(page, "desktop-boxoffice-aggregate");
  if (AFTER) {
    await page.getByRole("complementary", { name: "Inspector" }).evaluate((el) => el.scrollTo(0, el.scrollHeight));
    await shot(page, "desktop-boxoffice-aggregate-scrolled");
  }
});

test("empty recording on the map", async ({ page }) => {
  await openMap(page, "/empty/");
  await shot(page, "desktop-empty");
});

test("narrow: an aggregate selected on the map", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await openMap(page, "/");
  await page.getByRole("group", { name: "Entries of identityaccess" }).getByRole("button", { name: "Aggregate Tenant" }).click();
  await expect(page.getByRole("complementary", { name: "Inspector" }).getByRole("heading", { level: 1 })).toHaveText("Tenant");
  await page.getByRole("complementary", { name: "Inspector" }).scrollIntoViewIfNeeded();
  await shot(page, "narrow-identity-aggregate");
  if (!AFTER) return;
  await assess(page);
  const evidence = page.getByRole("region", { name: "Code evidence" });
  await evidence.scrollIntoViewIfNeeded();
  await shot(page, "narrow-identity-aggregate-evidence");
  await evidence.getByRole("button", { name: /^Peek at identityaccess\/tenant\/tenant\.go:\d+$/ }).first().click();
  const peek = page.getByRole("region", { name: /^Source peek: / });
  await expect(peek.locator(".st-peek__line.is-target")).toBeVisible();
  await peek.scrollIntoViewIfNeeded();
  await shot(page, "narrow-identity-aggregate-peek");
});

