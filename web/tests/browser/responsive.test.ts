import { expect, test } from "@playwright/test";
import { StudioPage } from "../support/studio-page";

test("keeps language lookup and editing usable on a narrow screen", async ({ page }) => {
  const studio = new StudioPage(page, "/boxoffice/");
  await studio.open();
  await page.screenshot({ path: "/tmp/arclint-studio-desktop.png" });
  await page.setViewportSize({ width: 390, height: 844 });
  await studio.search.fill("Order");
  await page.getByRole("option").filter({ has: page.getByRole("img", { name: "Aggregate", exact: true }) }).first().click();
  await expect(studio.card.getByRole("heading", { level: 1 })).toHaveText("Order");
  await studio.card.getByRole("button", { name: "Edit", exact: true }).click();
  await expect(studio.field("Definition")).toBeEditable();
  await studio.card.getByRole("button", { name: "Cancel" }).click();
  await page.getByRole("tab", { name: "Features" }).click();
  await page.getByRole("button", { name: "+ Feature" }).first().click();
  await page.getByRole("textbox", { name: "Title" }).fill("Reserve a seat");
  await page.locator(".st-editor-host .cm-content").click();
  await page.keyboard.type("An Order reserves a seat.");
  await page.locator(".st-use--cue", { hasText: "Order" }).click();
  await expect(page.getByRole("region", { name: "At the cursor" })).toContainText("Order");
  await page.screenshot({ path: "/tmp/arclint-studio-feature-mobile.png" });
  const horizontalOverflow = await page.locator("#studio").evaluate((host) => {
    const root = host.shadowRoot!.querySelector(".st-root")!;
    return root.scrollWidth > root.clientWidth;
  });
  await page.screenshot({ path: "/tmp/arclint-studio-mobile.png" });
  const outside = await page.locator("#studio").evaluate((host) => [...host.shadowRoot!.querySelectorAll("*")].filter((el) => el.getBoundingClientRect().right > 391).map((el) => ({ tag: el.tagName, cls: el.className, right: el.getBoundingClientRect().right })).slice(0, 20));
  expect(horizontalOverflow, JSON.stringify(outside)).toBe(false);
});
