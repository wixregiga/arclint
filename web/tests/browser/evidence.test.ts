import { expect, test, type Page } from "@playwright/test";
import { StudioPage } from "../support/studio-page";

/*
 * Code evidence in the map's inspector and the read-only source peek
 * below the map, through the real host over real recordings. The suite
 * fails on main, whose Studio has no code evidence.
 */

async function openMap(page: Page, base: string): Promise<StudioPage> {
  const studio = new StudioPage(page, base);
  await studio.open();
  await page.getByRole("tab", { name: "Map" }).click();
  return studio;
}

function inspector(page: Page) {
  return page.getByRole("complementary", { name: "Inspector" });
}

function evidence(page: Page) {
  return inspector(page).getByRole("region", { name: "Code evidence" });
}

async function selectOnMap(page: Page, context: string, label: string): Promise<void> {
  await page.getByRole("group", { name: `Entries of ${context}` }).getByRole("button", { name: label }).click();
}

async function assess(page: Page): Promise<void> {
  await evidence(page).getByRole("button", { name: "Assess code" }).click();
  await expect(evidence(page).getByText(/^Assessed /)).toBeVisible({ timeout: 60_000 });
}

/** The element focused inside Studio's shadow root, by its accessible label or text. */
function focusedLabel(page: Page): Promise<string | null> {
  return page.locator("#studio").evaluate((host) => {
    const active = host.shadowRoot!.activeElement as HTMLElement | null;
    return active ? active.getAttribute("aria-label") ?? active.textContent : null;
  });
}

test.describe("code evidence over the YAML recording", () => {
  test("assesses only when asked and shows declarations, carriers, readings and the whole assessment", async ({ page }) => {
    const asked: string[] = [];
    page.on("request", (r) => {
      if (r.url().includes("/api/evidence")) asked.push(r.url());
    });
    await openMap(page, "/");
    await selectOnMap(page, "identityaccess", "Aggregate Tenant");
    await expect(evidence(page)).toContainText("read-only");
    await expect(evidence(page).getByRole("button", { name: "Assess code" })).toBeVisible();
    await selectOnMap(page, "identityaccess", "Aggregate User");
    expect(asked, "selecting entries never assesses").toHaveLength(0);

    await selectOnMap(page, "identityaccess", "Aggregate Tenant");
    await assess(page);
    expect(asked).toHaveLength(1);
    const ev = evidence(page);
    await expect(ev.getByRole("button", { name: /^Peek at identityaccess\/tenant\/tenant\.go:\d+$/ }).first()).toBeVisible();
    await expect(ev.getByRole("button", { name: /^Peek at domain\.arclint\.yaml:\d+$/ })).toBeVisible();
    await expect(ev).toContainText("Found: presence, not proof.");
    await expect(ev).toContainText("one-open-invitation-per-description");
    await expect(ev).toContainText("structurally checked");
    // A context-level check that did not decide keeps every entry of the
    // context partial; it never reads as free of findings.
    await expect(ev.getByText("Partial", { exact: true })).toBeVisible();
    await expect(ev).toContainText("repository/not-called-from-aggregates");
    await expect(ev).not.toContainText("No attributed findings");
    const whole = ev.locator(".st-evidence__disclosure");
    await expect(whole).toContainText("coverage note(s)");
    await expect(whole).not.toContainText("every finding is tied to an entry");
    await expect(whole.locator("summary")).toContainText(/Not tied to any entry \(\d+ findings?, \d+ diagnostics?\)/);

    // The same document answers every selection without another assessment.
    await selectOnMap(page, "identityaccess", "Aggregate Role");
    await expect(ev).toContainText("members-listed-once");
    expect(asked).toHaveLength(1);
  });

  test("opens a read-only peek below the map, closes it with Escape and returns focus, and never steals focus on selection", async ({ page }) => {
    await openMap(page, "/");
    await selectOnMap(page, "identityaccess", "Aggregate Tenant");
    await assess(page);
    await page.locator("#studio").evaluate((host) => {
      (host.shadowRoot!.querySelector(".st-map") as HTMLElement).dataset.probe = "kept";
    });

    const declared = evidence(page).getByRole("button", { name: /^Peek at identityaccess\/tenant\/tenant\.go:\d+$/ }).first();
    const label = await declared.getAttribute("aria-label");
    await declared.click();
    const peek = page.getByRole("region", { name: /^Source peek: identityaccess\/tenant\/tenant\.go:\d+$/ });
    await expect(peek).toBeVisible();
    await expect(peek.locator(".st-peek__line.is-target")).toContainText("type Tenant struct");
    await expect(peek).toContainText("read-only");
    await expect(peek.locator("textarea, [contenteditable=true], input")).toHaveCount(0);
    expect(await focusedLabel(page)).toMatch(/^Source peek: /);

    // The peek sits below the map in the same main region.
    const main = page.locator("main.st-main--map");
    await expect(main.locator(".st-map")).toBeVisible();
    await expect(main.getByRole("region", { name: /^Source peek: / })).toBeVisible();

    await page.keyboard.press("Escape");
    await expect(peek).toHaveCount(0);
    expect(await focusedLabel(page)).toBe(label);

    // Selecting on the map keeps focus where the reader is, keeps an open
    // peek, and keeps the map mounted.
    await declared.click();
    await expect(peek).toBeVisible();
    const mapButton = page.getByRole("group", { name: "Entries of identityaccess" }).getByRole("button", { name: "Aggregate User" });
    await mapButton.click();
    await expect(inspector(page).getByRole("heading", { level: 1 })).toHaveText("User");
    expect(await focusedLabel(page)).toBe("Aggregate User");
    await expect(page.getByRole("region", { name: /^Source peek: identityaccess\/tenant\/tenant\.go/ })).toBeVisible();
    expect(await page.locator("#studio").evaluate((host) => (host.shadowRoot!.querySelector(".st-map") as HTMLElement).dataset.probe)).toBe("kept");

    // The close button closes it too.
    await page.getByRole("button", { name: "Close the source peek" }).click();
    await expect(page.getByRole("region", { name: /^Source peek: / })).toHaveCount(0);
  });

  test("marks the evidence stale after a reload and keeps it stale when assessing again fails", async ({ page }) => {
    await openMap(page, "/");
    await selectOnMap(page, "identityaccess", "Aggregate Tenant");
    await assess(page);
    const ev = evidence(page);
    await expect(ev.getByText("Stale", { exact: true })).toHaveCount(0);

    await page.getByRole("button", { name: "Reload" }).click();
    await expect(ev.getByText("Stale", { exact: true })).toBeVisible();
    await expect(ev).toContainText("may not match the code or the recording now");

    await page.route("**/api/evidence", (route) => route.fulfill({ status: 500, contentType: "application/json", body: JSON.stringify({ error: "assess code evidence: observe repository: disk vanished" }) }));
    await ev.getByRole("button", { name: "Assess again" }).click();
    await expect(ev.getByRole("alert")).toContainText("The assessment failed: assess code evidence: observe repository: disk vanished");
    await expect(ev.getByText("Stale", { exact: true })).toBeVisible();
    await expect(ev.getByRole("button", { name: /^Peek at identityaccess\/tenant\/tenant\.go:\d+$/ }).first()).toBeVisible();

    await page.unroute("**/api/evidence");
    await ev.getByRole("button", { name: "Assess again" }).click();
    await expect(ev.getByText("Stale", { exact: true })).toHaveCount(0);
    await expect(ev.getByRole("alert")).toHaveCount(0);
  });

  test("says when a peek's snapshot was replaced and when a file is withheld", async ({ page }) => {
    await openMap(page, "/");
    await selectOnMap(page, "identityaccess", "Aggregate Tenant");
    await assess(page);
    const location = evidence(page).getByRole("button", { name: /^Peek at identityaccess\/tenant\/tenant\.go:\d+$/ }).first();

    await page.route("**/api/source?**", (route) => route.fulfill({ status: 403, contentType: "application/json", body: JSON.stringify({ error: "withheld: the file may hold a secret: identityaccess/tenant/tenant.go" }) }));
    await location.click();
    await expect(page.getByRole("region", { name: /^Source peek: / })).toContainText("Withheld");
    await expect(page.locator(".st-peek__line")).toHaveCount(0);
    await page.unroute("**/api/source?**");

    await page.route("**/api/source?**", (route) => route.fulfill({ status: 409, contentType: "application/json", body: JSON.stringify({ error: "this evidence is stale: a newer assessment replaced it; assess again" }) }));
    await location.click();
    await expect(page.getByRole("region", { name: /^Source peek: / })).toContainText("A newer assessment replaced");
    await expect(evidence(page).getByText("Stale", { exact: true })).toBeVisible();
    await page.unroute("**/api/source?**");
  });

  test("reports a server without code evidence as unavailable, never as clean", async ({ page }) => {
    await page.route("**/api/evidence", (route) => route.fulfill({ status: 404, contentType: "text/plain", body: "404 page not found" }));
    await openMap(page, "/");
    await selectOnMap(page, "identityaccess", "Aggregate Tenant");
    await evidence(page).getByRole("button", { name: "Assess code" }).click();
    await expect(evidence(page)).toContainText("This map server serves no code evidence.");
    await expect(evidence(page)).not.toContainText("No attributed findings");
    await expect(evidence(page).getByRole("button", { name: "Assess code" })).toHaveCount(0);
  });

  test("refuses excerpt requests the evidence does not name", async ({ page }) => {
    await openMap(page, "/");
    const answers = await page.evaluate(async () => {
      const base = new URL("./", document.baseURI);
      const evidence = await (await fetch(new URL("api/evidence", base), { cache: "no-store" })).json();
      const ask = async (query: Record<string, string>) => (await fetch(new URL(`api/source?${new URLSearchParams(query)}`, base), { cache: "no-store" })).status;
      const snapshot = evidence.snapshot as string;
      return {
        traversal: await ask({ snapshot, path: "../../../../etc/passwd", line: "1" }),
        absolute: await ask({ snapshot, path: "/etc/passwd", line: "1" }),
        manifest: await ask({ snapshot, path: "go.mod", line: "1" }),
        unknown: await ask({ snapshot: "0".repeat(32), path: "identityaccess/tenant/tenant.go", line: "1" }),
        source: await ask({ snapshot, path: "identityaccess/tenant/tenant.go", line: "16" }),
      };
    });
    expect(answers).toEqual({ traversal: 400, absolute: 400, manifest: 404, unknown: 409, source: 200 });
  });
});

test("assesses a second, unrelated recording and lists its findings with what ties them", async ({ page }) => {
  await openMap(page, "/boxoffice/");
  await page.getByRole("group", { name: "Bounded contexts and their relations" }).getByRole("button", { name: "Context ordering" }).click();
  await selectOnMap(page, "ordering", "Aggregate Order");
  await assess(page);
  const ev = evidence(page);
  await expect(ev).toContainText("internal/entities/order/order.go");
  await expect(ev.getByText("baselined").first()).toBeVisible();
  await expect(ev).toContainText("tied through its contract and a file declaring it");

  await page.getByRole("group", { name: "Bounded contexts and their relations" }).getByRole("button", { name: "Context catalog" }).click();
  await inspector(page).getByRole("heading", { level: 1 }).waitFor();
  await selectOnMap(page, "catalog", "Aggregate Event");
  await expect(ev).toContainText("internal/entities/event/event.go");
});

test("answers an empty recording's failed check with its reason, never with an empty clean result", async ({ page }) => {
  await openMap(page, "/empty/");
  await expect(page.getByRole("region", { name: "Code evidence" })).toHaveCount(0);
  // The empty fixture's ruleset names a key the rule loader refuses, so
  // `arclint check` fails there too; the evidence says why.
  const answered = await page.evaluate(async () => {
    const r = await fetch(new URL("api/evidence", document.baseURI), { cache: "no-store" });
    return { status: r.status, body: (await r.json()) as { entries?: unknown[]; error?: string } };
  });
  expect(answered.status).toBe(500);
  expect(answered.body.entries).toBeUndefined();
  expect(answered.body.error).toContain("load configured rules");
});

test("keeps the peek inside the map region on a narrow screen", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await openMap(page, "/");
  await selectOnMap(page, "identityaccess", "Aggregate Tenant");
  await assess(page);
  await evidence(page).getByRole("button", { name: /^Peek at identityaccess\/tenant\/tenant\.go:\d+$/ }).first().click();
  await expect(page.locator(".st-peek__line.is-target")).toBeVisible();
  const layout = await page.locator("#studio").evaluate((host) => {
    const box = (s: string) => host.shadowRoot!.querySelector(s)!.getBoundingClientRect();
    const root = host.shadowRoot!.querySelector(".st-root")!;
    return { nav: box(".st-navigator"), main: box(".st-main--map"), map: box(".st-map"), peek: box(".st-peek"), overflow: root.scrollWidth > root.clientWidth };
  });
  expect(layout.overflow).toBe(false);
  expect(layout.nav.bottom).toBeLessThanOrEqual(layout.map.top + 0.5);
  expect(layout.peek.top).toBeGreaterThanOrEqual(layout.map.bottom - 0.5);
  expect(layout.peek.bottom).toBeLessThanOrEqual(layout.main.bottom + 0.5);
  expect(layout.peek.right).toBeLessThanOrEqual(391);
});
