/*
 * The pull request's walkthrough: real footage of the built Studio over
 * the identity example, recorded by Playwright, nothing staged.
 *
 *   EVIDENCE_VIDEO=1 EVIDENCE_DIR=<dir> ARCLINT_BIN=<binary> npx playwright test -c playwright.evidence.config.ts walkthrough
 */
import path from "node:path";
import { expect, test } from "@playwright/test";
import { StudioPage } from "../support/studio-page";

const OUT = process.env.EVIDENCE_DIR ?? "";

test.skip(OUT === "" || process.env.EVIDENCE_VIDEO !== "1", "EVIDENCE_VIDEO=1 and EVIDENCE_DIR record the walkthrough");
test.use({ viewport: { width: 1280, height: 720 }, video: { mode: "on", size: { width: 1280, height: 720 } } });

async function openMap(page: import("@playwright/test").Page, base: string): Promise<void> {
  await new StudioPage(page, base).open();
  await page.getByRole("tab", { name: "Map" }).click();
}

test("assess, peek, close with Escape, move selection, assess again", async ({ page }) => {
  const pause = (ms: number) => page.waitForTimeout(ms);
  await openMap(page, "/");
  await pause(800);
  const entries = page.getByRole("group", { name: "Entries of identityaccess" });
  await entries.getByRole("button", { name: "Aggregate Tenant" }).click();
  await pause(900);
  const inspector = page.getByRole("complementary", { name: "Inspector" });
  const evidence = inspector.getByRole("region", { name: "Code evidence" });
  await evidence.scrollIntoViewIfNeeded();
  await pause(900);
  await evidence.getByRole("button", { name: "Assess code" }).click();
  await expect(evidence.getByText(/^Assessed /)).toBeVisible({ timeout: 60_000 });
  await pause(1500);
  const declared = evidence.getByRole("button", { name: /^Peek at identityaccess\/tenant\/tenant\.go:\d+$/ }).first();
  await declared.click();
  await expect(page.getByRole("region", { name: /^Source peek: / }).locator(".st-peek__line.is-target")).toBeVisible();
  await pause(2000);
  await evidence.getByRole("button", { name: /^Peek at identityaccess\/tenant\/tenant\.go:\d+$/ }).nth(1).click();
  await pause(2000);
  await page.keyboard.press("Escape");
  await expect(page.getByRole("region", { name: /^Source peek: / })).toHaveCount(0);
  await pause(900);
  await inspector.evaluate((el) => el.scrollTo({ top: el.scrollHeight }));
  await pause(1500);
  await entries.getByRole("button", { name: "Aggregate User" }).click();
  await pause(900);
  await inspector.getByRole("region", { name: "Code evidence" }).scrollIntoViewIfNeeded();
  await pause(1500);
  await inspector.getByRole("region", { name: "Code evidence" }).getByRole("button", { name: "Assess again" }).click();
  await expect(inspector.getByRole("region", { name: "Code evidence" }).getByText(/^Assessed /)).toBeVisible({ timeout: 60_000 });
  await pause(1500);
  const video = page.video();
  await page.close();
  if (video) await video.saveAs(path.join(OUT, "walkthrough.webm"));
});
