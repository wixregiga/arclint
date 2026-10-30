import { expect, test } from "@playwright/test";
import { StudioPage } from "../support/studio-page";

test.describe("editing an entry", () => {
  test("saves a changed definition, keeps every other property, and stands Proposed against origin/main", async ({ page }) => {
    const studio = new StudioPage(page, "/");
    await studio.open();
    await studio.select("Person");
    await studio.card.getByRole("button", { name: "Edit", exact: true }).click();
    const definition = studio.field("Definition");
    await expect(definition).toHaveValue(/FullName and EmailAddress/);
    await definition.fill("The FullName and EmailAddress of the person behind a user, kept under that user's tenant.");
    await studio.card.getByRole("button", { name: "Save" }).click();

    await expect(studio.card.getByRole("form")).toHaveCount(0);
    await expect(studio.card).toContainText("kept under that user's tenant");
    await expect(studio.card).toContainText("Proposed");
    await expect(studio.card).toContainText("differs from origin/main: definition");
    await expect(page.getByRole("status")).toContainText(/updated value object Person/);

    const overview = await studio.overview();
    const person = overview.contexts![0].valueObjects!.find((v) => v.name === "Person")!;
    expect(person.definition).toContain("kept under that user's tenant");
    const tenant = overview.contexts![0].aggregates!.find((a) => a.name === "Tenant")!;
    expect(tenant.identity).toBe("TenantID");
    expect(tenant.repository).toBe("TenantRepository");
    expect(tenant.entities!.map((e) => e.name)).toEqual(["RegistrationInvitation"]);

    await page.reload();
    await expect(studio.card).toContainText("kept under that user's tenant");
    await expect(studio.card.getByRole("heading", { level: 1 })).toHaveText("Person");
  });

  test("records aliases and finds the entry by one", async ({ page }) => {
    const studio = new StudioPage(page, "/");
    await studio.open();
    await studio.select("FullName");
    await studio.card.getByRole("button", { name: "Edit", exact: true }).click();
    await studio.field("Aliases").fill("Display name\nName");
    await studio.card.getByRole("button", { name: "Save" }).click();
    await expect(studio.card.getByText("Display name")).toBeVisible();
    await studio.search.fill("display name");
    await expect(page.getByRole("option").first()).toContainText("FullName");
    await expect(page.getByRole("option").first()).toContainText("also “Display name”");
    const overview = await studio.overview();
    expect(overview.contexts![0].valueObjects!.find((v) => v.name === "FullName")!.aliases).toEqual(["Display name", "Name"]);
  });

  test("shows the language's refusal in its own words and keeps the typed text", async ({ page }) => {
    const studio = new StudioPage(page, "/");
    await studio.open();
    await studio.select("identityaccess");
    await studio.card.getByRole("button", { name: "Edit", exact: true }).click();
    const tooLong = Array.from({ length: 70 }, (_, i) => `word${i}`).join(" ");
    await studio.field("Definition").fill(tooLong);
    await studio.card.getByRole("button", { name: "Save" }).click();
    const alert = studio.card.getByRole("alert");
    await expect(alert).toBeVisible();
    await expect(alert).toContainText(/word/);
    await expect(studio.field("Definition")).toHaveValue(tooLong);
    const overview = await studio.overview();
    expect(overview.contexts![0].definition).not.toContain("word0");
    await studio.card.getByRole("button", { name: "Cancel" }).click();
    await expect(studio.card).toContainText("Identity and access for the SaaSOvation tenants");
  });

  test("offers to try again when the server cannot be reached, without losing the edit", async ({ page }) => {
    const studio = new StudioPage(page, "/");
    await studio.open();
    await studio.select("Username");
    await studio.card.getByRole("button", { name: "Edit", exact: true }).click();
    await studio.field("Definition").fill("The name a user signs in with; chosen once.");
    await page.route("**/api/define", (route) => route.abort("connectionrefused"));
    await studio.card.getByRole("button", { name: "Save" }).click();
    await expect(studio.card.getByRole("alert")).toContainText("did not answer");
    await expect(page.getByRole("button", { name: "Try again" })).toBeVisible();
    await expect(studio.field("Definition")).toHaveValue("The name a user signs in with; chosen once.");
    await page.unroute("**/api/define");
    await page.getByRole("button", { name: "Try again" }).click();
    await expect(studio.card.getByRole("form")).toHaveCount(0);
    await expect(studio.card).toContainText("chosen once");
  });

  test("keeps an unsaved draft across the map, another entry and a reload", async ({ page }) => {
    const studio = new StudioPage(page, "/");
    await studio.open();
    await studio.select("Password");
    await studio.card.getByRole("button", { name: "Edit", exact: true }).click();
    await studio.field("Definition").fill("A password as a user chose it, before protection. Draft text.");
    await page.getByRole("tab", { name: "Map" }).click();
    await expect(page.getByRole("group", { name: "Entries of identityaccess" })).toBeVisible();
    await page.getByRole("tab", { name: "Language" }).click();
    await studio.select("Credentials");
    await expect(studio.navigator.getByRole("button", { name: /Password.*unsaved/ })).toBeVisible();
    await page.reload();
    await studio.select("Password");
    await expect(studio.card.getByRole("status")).toContainText("unsaved changes");
    await studio.card.getByRole("button", { name: "Continue editing" }).click();
    await expect(studio.field("Definition")).toHaveValue(/Draft text\.$/);
    await studio.card.getByRole("button", { name: "Cancel" }).click();
    await expect(studio.card).not.toContainText("Draft text");
    await expect(studio.navigator.getByRole("button", { name: /Password.*unsaved/ })).toHaveCount(0);
  });
});


test("keeps the draft when a successful write cannot be read back yet", async ({ page }) => {
  const studio = new StudioPage(page, "/");
  await studio.open();
  await studio.select("Credentials");
  await studio.card.getByRole("button", { name: "Edit", exact: true }).click();
  const changed = "A username and password presented together to authenticate a user.";
  await studio.field("Definition").fill(changed);
  await page.route("**/api/overview", (route) => route.abort("connectionrefused"));
  await studio.card.getByRole("button", { name: "Save" }).click();
  await expect(page.getByRole("contentinfo").getByRole("alert")).toContainText("did not answer");
  await expect(studio.field("Definition")).toHaveValue(changed);
  await page.unroute("**/api/overview");
  await page.getByRole("button", { name: "Try again" }).click();
  expect((await studio.overview()).contexts![0].valueObjects!.find((v) => v.name === "Credentials")!.definition).toBe(changed);
  await studio.card.getByRole("button", { name: "Cancel" }).click();
  await expect(studio.card).toContainText(changed);
});
