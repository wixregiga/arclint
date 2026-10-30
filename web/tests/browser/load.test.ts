import { expect, test } from "@playwright/test";
import { StudioPage } from "../support/studio-page";

test.describe("loading the recorded language", () => {
  test("shows the project, its contexts, and an entry as recorded, from the origin root", async ({ page }) => {
    const studio = new StudioPage(page, "/");
    await studio.open();
    await expect(page.getByRole("button", { name: "identityaccess" }).first()).toBeVisible();
    await expect(studio.card.getByRole("heading", { level: 1 })).toHaveText("identityaccess");
    await expect(studio.card).toContainText("Identity and access for the SaaSOvation tenants");
    await expect(studio.navigator.getByRole("region", { name: "Context identityaccess" })).toBeVisible();

    await studio.select("Tenant");
    await expect(studio.card.getByRole("heading", { level: 1 })).toHaveText("Tenant");
    await expect(studio.card).toContainText("A subscribing organisation that owns users and roles");
    await expect(studio.card.getByRole("button", { name: "TenantID", exact: true })).toBeVisible();
    await expect(studio.card.getByRole("region", { name: "Connections" })).toContainText("RegistrationInvitation");
    await expect(studio.card.getByRole("region", { name: "Connections" })).toContainText("Identified by");
    await expect(studio.card).toContainText("Accepted");

    await studio.card.getByRole("region", { name: "Connections" }).getByRole("button", { name: /RegistrationInvitation/ }).click();
    await expect(studio.card.getByRole("heading", { level: 1 })).toHaveText("RegistrationInvitation");
    await expect(studio.card).toContainText("identityaccess › Tenant");
  });

  test("keeps every request under a nested base", async ({ page }) => {
    const api: string[] = [];
    page.on("request", (r) => {
      if (r.url().includes("/api/")) api.push(new URL(r.url()).pathname);
    });
    const studio = new StudioPage(page, "/nested/studio/");
    await studio.open();
    await expect(studio.card.getByRole("heading", { level: 1 })).toHaveText("identityaccess");
    expect(api.length).toBeGreaterThanOrEqual(2);
    for (const p of api) expect(p.startsWith("/nested/studio/api/")).toBe(true);
    const manifest = await page.evaluate(() => (document.querySelector('link[rel="manifest"]') as HTMLLinkElement).href);
    expect(new URL(manifest).pathname).toBe("/nested/studio/manifest.webmanifest");
  });

  test("finds entries by name, alias and definition and goes to them", async ({ page }) => {
    const studio = new StudioPage(page, "/");
    await studio.open();
    await studio.search.fill("tenant");
    const hits = page.getByRole("option");
    await expect(hits.first()).toContainText("Tenant");
    await expect(page.getByRole("listbox", { name: "Entries found" })).toContainText("TenantID");
    await studio.search.press("ArrowDown");
    await studio.search.press("Enter");
    await expect(studio.card.getByRole("heading", { level: 1 })).not.toHaveText("identityaccess");
    await expect(studio.search).toHaveValue("");

    await studio.search.fill("mailbox address");
    await expect(page.getByRole("option").first()).toContainText("well-formed");
    await studio.search.press("Enter");
    await expect(studio.card.getByRole("heading", { level: 1 })).toHaveText("well-formed");
    await expect(studio.card).toContainText("Always holds for");

    await studio.search.fill("zzz-nothing-here");
    await expect(page.getByRole("option")).toContainText(["Nothing recorded matches"]);
    await studio.search.press("Escape");
    await expect(studio.search).toHaveValue("");
  });

  test("reports a server that cannot be reached and reads again on request", async ({ page }) => {
    await page.route("**/api/overview", (route) => route.abort("connectionrefused"));
    const studio = new StudioPage(page, "/");
    await page.goto("/");
    await expect(page.getByRole("alert")).toContainText("Studio cannot read the recording");
    await page.unroute("**/api/overview");
    await page.getByRole("button", { name: "Try again" }).click();
    await expect(studio.navigator).toBeVisible();
    await expect(studio.card.getByRole("heading", { level: 1 })).toHaveText("identityaccess");
  });
});
