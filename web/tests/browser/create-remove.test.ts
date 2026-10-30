import { expect, test } from "@playwright/test";
import { StudioPage } from "../support/studio-page";

test.describe("beginning a language in an empty recording", () => {
  test("records a context, an aggregate, an entity and a question, then removes them", async ({ page }) => {
    const studio = new StudioPage(page, "/empty/");
    await studio.open();
    await expect(studio.card.getByRole("heading", { level: 1 })).toHaveText("empty");
    await expect(studio.navigator).toContainText("No bounded context is recorded yet");
    await expect(page.getByText("Default branch unavailable")).toBeVisible();

    await studio.navigator.getByRole("button", { name: "+ Context" }).click();
    await expect(studio.card.getByRole("form", { name: "New Bounded context" })).toBeVisible();
    await studio.field("Name").fill("lending");
    await expect(studio.card).toContainText("Still needed before it can be recorded: definition");
    await studio.field("Definition").fill("Lending books to members and getting them back.");
    await studio.card.getByRole("button", { name: "Record" }).click();
    await expect(studio.card.getByRole("heading", { level: 1 })).toHaveText("lending");
    await expect(studio.navigator.getByRole("region", { name: "Context lending" })).toBeVisible();

    await studio.navigator.getByRole("button", { name: "Record in lending" }).click();
    await studio.navigator.getByRole("menuitem", { name: /Aggregate/ }).click();
    await studio.field("Name").fill("Loan");
    await studio.field("Definition").fill("One member borrowing one book until it is returned.");
    await studio.card.getByRole("button", { name: "Record" }).click();
    await expect(studio.card.getByRole("alert")).toContainText("needs identity");
    await expect(studio.field("Definition")).toHaveValue("One member borrowing one book until it is returned.");
    await studio.field("Identity").fill("LoanID");
    await studio.card.getByRole("button", { name: "Record" }).click();
    await expect(studio.card.getByRole("heading", { level: 1 })).toHaveText("Loan");
    await expect(studio.card).toContainText("LoanID");

    await studio.card.getByRole("button", { name: "+ Entity" }).click();
    await studio.field("Name").fill("Renewal");
    await studio.field("Definition").fill("One extension of a loan's due date.");
    await studio.card.getByRole("button", { name: "Record" }).click();
    await expect(studio.card.getByRole("heading", { level: 1 })).toHaveText("Renewal");
    await expect(studio.card).toContainText("lending › Loan");

    await studio.select("Loan");
    await expect(studio.card.getByRole("region", { name: "Connections" })).toContainText("Renewal");
    await studio.card.getByRole("button", { name: "+ Invariant" }).click();
    await studio.field("Key").fill("one-open-loan-per-copy");
    await studio.field("Statement").fill("A copy is on at most one open loan.");
    await studio.card.getByRole("button", { name: "Record" }).click();
    await expect(studio.card.getByRole("heading", { level: 1 })).toHaveText("one-open-loan-per-copy");

    await studio.navigator.getByRole("button", { name: "Record in lending" }).click();
    await studio.navigator.getByRole("menuitem", { name: /Open question/ }).click();
    await studio.field("Key").fill("late-fees");
    await studio.field("Question").fill("Do we charge late fees at all?");
    await studio.card.getByRole("button", { name: "Record" }).click();
    await expect(studio.card.getByRole("heading", { level: 1 })).toHaveText("late-fees");

    let overview = await studio.overview();
    expect(overview.contexts![0].name).toBe("lending");
    expect(overview.contexts![0].aggregates![0].entities!.map((e) => e.name)).toEqual(["Renewal"]);
    expect(overview.contexts![0].aggregates![0].invariants!.map((i) => i.key)).toEqual(["one-open-loan-per-copy"]);
    expect(overview.contexts![0].questions![0].text).toBe("Do we charge late fees at all?");

    await studio.select("Renewal", "Loan");
    await studio.card.getByRole("button", { name: "Remove…" }).click();
    await studio.card.getByRole("button", { name: "Remove Renewal" }).click();
    await expect(studio.card.getByRole("heading", { level: 1 })).toHaveText("Loan");
    await expect(studio.card.getByRole("region", { name: "Connections" })).not.toContainText("Renewal");

    await studio.select("lending");
    await studio.card.getByRole("button", { name: "Remove…" }).click();
    await expect(studio.card.getByRole("group", { name: "Remove lending" })).toContainText("Loan");
    await studio.card.getByRole("button", { name: "Keep" }).click();
    await studio.card.getByRole("button", { name: "Remove…" }).click();
    await studio.card.getByRole("button", { name: "Remove lending" }).click();
    await expect(studio.card.getByRole("heading", { level: 1 })).toHaveText("empty");
    await expect(studio.navigator).toContainText("No bounded context is recorded yet");
    overview = await studio.overview();
    expect(overview.contexts ?? []).toEqual([]);
  });

  test("describes the project through api/project when the server offers it", async ({ page }) => {
    const studio = new StudioPage(page, "/empty/");
    await studio.open();
    test.skip(!(await studio.hasProjectRoute()), "api/project is not served by this arclint binary");
    await studio.card.getByRole("button", { name: "Edit", exact: true }).click();
    await studio.field("Description").fill("A small library.");
    await studio.card.getByRole("button", { name: "Save" }).click();
    await expect(studio.card).toContainText("A small library.");
    const overview = await studio.overview();
    expect(overview.description).toBe("A small library.");
    expect(overview.project).toBe("empty");
  });

  test("reports a project edit the running server cannot take, keeping the text", async ({ page }) => {
    const studio = new StudioPage(page, "/empty/");
    await studio.open();
    test.skip(await studio.hasProjectRoute(), "api/project is served; the success path covers it");
    await studio.card.getByRole("button", { name: "Edit", exact: true }).click();
    await studio.field("Description").fill("A small library.");
    await studio.card.getByRole("button", { name: "Save" }).click();
    await expect(studio.card.getByRole("alert")).toContainText("404");
    await expect(studio.field("Description")).toHaveValue("A small library.");
  });
});
