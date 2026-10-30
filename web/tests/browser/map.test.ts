import { expect, test } from "@playwright/test";
import { StudioPage } from "../support/studio-page";

test.describe("the map over a second, unrelated recording", () => {
  test("draws the context map and a context's connections from the CML recording and opens the inspector", async ({ page }) => {
    const studio = new StudioPage(page, "/boxoffice/");
    await studio.open();
    await expect(studio.card.getByRole("heading", { level: 1 })).toHaveText("boxoffice");
    await expect(studio.card).toContainText("Not available:");
    await expect(page.getByText("Default branch unavailable")).toBeVisible();
    for (const c of ["catalog", "ordering", "capacity"]) await expect(studio.navigator.getByRole("region", { name: `Context ${c}` })).toBeVisible();

    await page.getByRole("tab", { name: "Map" }).click();
    const contextMap = page.getByRole("group", { name: "Bounded contexts and their relations" });
    await expect(contextMap).toBeVisible();
    await expect(contextMap.locator(".st-context-map__relation")).toHaveCount(3);
    await expect(contextMap).toContainText("customer supplier");

    await contextMap.getByRole("button", { name: "Context ordering" }).click();
    const entries = page.getByRole("group", { name: "Entries of ordering" });
    await expect(entries).toBeVisible();
    expect(await entries.locator(".st-concept-map__line--owner").count()).toBeGreaterThan(0);
    await expect(page.getByRole("complementary", { name: "Inspector" })).toContainText("ordering");

    await entries.getByRole("button", { name: "Aggregate Order" }).click();
    const inspector = page.getByRole("complementary", { name: "Inspector" });
    await expect(inspector.getByRole("heading", { level: 1 })).toHaveText("Order");
    await expect(inspector.getByRole("region", { name: "Connections" })).toBeVisible();
    await expect(inspector).not.toContainText("Accepted");

    await inspector.getByRole("button", { name: "Edit", exact: true }).click();
    await expect(inspector.getByRole("form", { name: "Edit Aggregate Order" })).toBeVisible();
    await inspector.getByRole("button", { name: "Cancel" }).click();
    await page.getByRole("tab", { name: "Language" }).click();
    await expect(studio.card.getByRole("heading", { level: 1 })).toHaveText("Order");
  });

  test("shows the YAML recording's connections on its map and its standing", async ({ page }) => {
    const studio = new StudioPage(page, "/");
    await studio.open();
    await page.getByRole("tab", { name: "Map" }).click();
    const entries = page.getByRole("group", { name: "Entries of identityaccess" });
    await expect(entries).toBeVisible();
    expect(await entries.locator(".st-concept-map__line--identity").count()).toBeGreaterThan(0);
    expect(await entries.locator(".st-concept-map__line--raises").count()).toBeGreaterThan(0);
    await entries.getByRole("button", { name: "Value object Person" }).click();
    const inspector = page.getByRole("complementary", { name: "Inspector" });
    await expect(inspector.getByRole("heading", { level: 1 })).toHaveText("Person");
  });
});
