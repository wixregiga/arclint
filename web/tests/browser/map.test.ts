import { expect, test } from "@playwright/test";
import { StudioPage } from "../support/studio-page";

test("renders arrowheads for supplied mutual, separate and one-way relations", async ({ page }) => {
  await page.route("**/boxoffice/api/overview", async (route) => {
    const response = await route.fetch();
    const overview = await response.json();
    overview.relations = [
      { from: "catalog", to: "ordering", kind: "partnership" },
      { from: "ordering", to: "capacity", kind: "separate_ways" },
      { from: "catalog", to: "capacity", kind: "conformist" },
    ];
    await route.fulfill({ response, json: overview });
  });
  await new StudioPage(page, "/boxoffice/").open();
  await page.getByRole("tab", { name: "Map" }).click();
  const relations = page.locator(".st-context-map__relation");
  const mutual = relations.filter({ hasText: "partnership" }).locator("line");
  await expect(mutual).toHaveAttribute("marker-start", "url(#st-arrow)");
  await expect(mutual).toHaveAttribute("marker-end", "url(#st-arrow)");
  const separate = relations.filter({ hasText: "separate ways" }).locator("line");
  await expect(separate).not.toHaveAttribute("marker-start");
  await expect(separate).not.toHaveAttribute("marker-end");
  await expect(separate).toContainText("separate ways from");
  const oneWay = relations.filter({ hasText: "conformist" }).locator("line");
  await expect(oneWay).not.toHaveAttribute("marker-start");
  await expect(oneWay).toHaveAttribute("marker-end", "url(#st-arrow)");
  await expect(oneWay).toContainText("upstream of");
});

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
