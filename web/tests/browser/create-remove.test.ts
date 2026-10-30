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
    await expect(studio.card.getByRole("form")).toHaveCount(0);
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
    await expect(studio.card.getByRole("form")).toHaveCount(0);
    await expect(studio.card.getByRole("heading", { level: 1 })).toHaveText("Loan");
    await expect(studio.card).toContainText("LoanID");

    await studio.card.getByRole("button", { name: "+ Entity" }).click();
    await studio.field("Name").fill("Renewal");
    await studio.field("Definition").fill("One extension of a loan's due date.");
    await studio.card.getByRole("button", { name: "Record" }).click();
    await expect(studio.card.getByRole("form")).toHaveCount(0);
    await expect(studio.card.getByRole("heading", { level: 1 })).toHaveText("Renewal");
    await expect(studio.card).toContainText("lending › Loan");

    await studio.select("Loan");
    await expect(studio.card.getByRole("region", { name: "Connections" })).toContainText("Renewal");
    await studio.card.getByRole("button", { name: "+ Invariant" }).click();
    await studio.field("Key").fill("one-open-loan-per-copy");
    await studio.field("Statement").fill("A copy is on at most one open loan.");
    await studio.card.getByRole("button", { name: "Record" }).click();
    await expect(studio.card.getByRole("form")).toHaveCount(0);
    await expect(studio.card.getByRole("heading", { level: 1 })).toHaveText("one-open-loan-per-copy");

    await studio.navigator.getByRole("button", { name: "Record in lending" }).click();
    await studio.navigator.getByRole("menuitem", { name: /Open question/ }).click();
    await studio.field("Key").fill("late-fees");
    await studio.field("Question").fill("Do we charge late fees at all?");
    let releaseQuestion!: () => void;
    let questionStarted!: () => void;
    const questionHeld = new Promise<void>((done) => { releaseQuestion = done; });
    const questionReached = new Promise<void>((done) => { questionStarted = done; });
    await page.route("**/empty/api/define", async (route) => {
      questionStarted();
      await questionHeld;
      await route.continue();
    });
    await studio.card.getByRole("button", { name: "Record" }).click();
    await questionReached;
    try {
      // The draft heading already names the question while its write is held.
      await expect(studio.card.getByRole("heading", { level: 1 })).toHaveText("late-fees");
      await expect(studio.card.getByRole("form", { name: "New Open question" })).toBeVisible();
      expect((await studio.overview()).contexts![0].questions ?? []).toEqual([]);
    } finally {
      releaseQuestion();
    }
    await expect(studio.card.getByRole("form")).toHaveCount(0);
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

  test("describes the project through api/project", async ({ page }) => {
    const studio = new StudioPage(page, "/empty/");
    await studio.open();
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
    await page.route("**/api/project", (route) => route.fulfill({ status: 404, contentType: "application/json", body: JSON.stringify({ error: "Project endpoint is unavailable (404)." }) }));
    await studio.card.getByRole("button", { name: "Edit", exact: true }).click();
    await studio.field("Description").fill("A library with home delivery.");
    await studio.card.getByRole("button", { name: "Save" }).click();
    await expect(studio.card.getByRole("alert")).toContainText("404");
    await expect(studio.field("Description")).toHaveValue("A library with home delivery.");
  });
});


test("begins a language with no domain file through the embedded Studio", async ({ page }) => {
  const studio = new StudioPage(page, "/new/");
  await studio.open();
  expect((await studio.overview()).found).toBe(false);
  await expect(studio.card).toContainText("No domain file exists");
  await studio.card.getByRole("button", { name: "Name the project" }).click();
  await studio.field("Project name").fill("Neighbourhood library");
  await studio.field("Description").fill("Loans and returns for our neighbourhood library.");
  await studio.card.getByRole("button", { name: "Begin the language" }).click();
  await expect(studio.card.getByRole("heading", { level: 1 })).toHaveText("Neighbourhood library");
  expect((await studio.overview()).found).toBe(true);
  await studio.card.getByRole("button", { name: "+ Bounded context" }).click();
  await studio.field("Name").fill("lending");
  await studio.field("Definition").fill("Lending books to members.");
  await studio.card.getByRole("button", { name: "Record", exact: true }).click();
  await expect(studio.card.getByRole("form")).toHaveCount(0);
  await expect(studio.card.getByRole("heading", { level: 1 })).toHaveText("lending");
  await page.reload();
  await expect(studio.card.getByRole("heading", { level: 1 })).toHaveText("lending");
  expect((await studio.overview()).contexts![0].definition).toBe("Lending books to members.");
});
