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

  test("marks evidence stale when the recording changes while its assessment is still running", async ({ page }) => {
    let release: () => void = () => {};
    const held = new Promise<void>((resolve) => (release = resolve));
    await page.route("**/api/evidence", async (route) => {
      const response = await route.fetch();
      await held;
      await route.fulfill({ response });
    });
    await openMap(page, "/");
    await selectOnMap(page, "identityaccess", "Aggregate Tenant");
    await evidence(page).getByRole("button", { name: "Assess code" }).click();
    await expect(evidence(page).getByRole("button", { name: "Assessing the code…" })).toBeVisible();
    // The reader reloads the recording while the assessment is still on its way.
    await page.getByRole("button", { name: "Reload" }).click();
    await expect(page.getByRole("status").filter({ hasText: "Read the domain file again." })).toBeVisible();
    release();
    await expect(evidence(page).getByText(/^Assessed /)).toBeVisible();
    await expect(evidence(page).getByText("Stale", { exact: true })).toBeVisible();
    await expect(evidence(page)).toContainText("The domain file was read again after this assessment");
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
    const r = await fetch(new URL("api/evidence", document.baseURI), { cache: "no-store", headers: { "X-ArcLint-Studio": "1" } });
    return { status: r.status, body: (await r.json()) as { entries?: unknown[]; error?: string } };
  });
  expect(answered.status).toBe(500);
  expect(answered.body.entries).toBeUndefined();
  expect(answered.body.error).toContain("load configured rules");
});

test("keeps the selected entry, its relationships and the source line in view together on a narrow screen", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await openMap(page, "/");
  // The map folds the navigator behind a labelled toggle.
  const toggle = page.getByRole("button", { name: /^Show the language list \(\d+\)$/ });
  await expect(toggle).toHaveAttribute("aria-expanded", "false");
  await expect(page.getByRole("navigation", { name: "The recorded language" })).toBeHidden();
  await toggle.click();
  await expect(page.getByRole("button", { name: "Hide the language list" })).toHaveAttribute("aria-expanded", "true");
  await expect(page.getByRole("navigation", { name: "The recorded language" })).toBeVisible();
  await page.getByRole("button", { name: "Hide the language list" }).click();

  await selectOnMap(page, "identityaccess", "Aggregate Tenant");
  await assess(page);
  await evidence(page).getByRole("button", { name: /^Peek at identityaccess\/tenant\/tenant\.go:\d+$/ }).first().click();
  await expect(page.locator(".st-peek__line.is-target")).toBeVisible();
  const layout = await page.locator("#studio").evaluate((host) => {
    const root = host.shadowRoot!;
    const box = (el: Element) => el.getBoundingClientRect();
    const stage = box(root.querySelector(".st-map__stage")!);
    const node = box(root.querySelector(".st-concept-map__node.is-selected circle")!);
    const code = box(root.querySelector(".st-peek__code")!);
    const target = box(root.querySelector(".st-peek__line.is-target")!);
    const path = root.querySelector(".st-peek__path")!;
    const file = box(path.querySelector(".st-path__file")!);
    const lines = [...root.querySelectorAll(".st-concept-map__line")].map(box).filter((l) => l.right > stage.left && l.left < stage.right && l.bottom > stage.top && l.top < stage.bottom).length;
    const html = root.querySelector(".st-root")!;
    return { stage, node, code, target, file, lines, head: box(root.querySelector(".st-peek__head")!), main: box(root.querySelector(".st-main--map")!), peek: box(root.querySelector(".st-peek")!), overflow: html.scrollWidth > html.clientWidth };
  });
  expect(layout.overflow).toBe(false);
  // The selected entry and some of its relationships are inside the visible stage.
  expect(layout.node.left).toBeGreaterThanOrEqual(layout.stage.left);
  expect(layout.node.right).toBeLessThanOrEqual(layout.stage.right);
  expect(layout.node.top).toBeGreaterThanOrEqual(layout.stage.top);
  expect(layout.node.bottom).toBeLessThanOrEqual(layout.stage.bottom);
  expect(layout.lines).toBeGreaterThan(0);
  // The highlighted line is inside the visible code at the same time.
  expect(layout.target.top).toBeGreaterThanOrEqual(layout.code.top - 0.5);
  expect(layout.target.bottom).toBeLessThanOrEqual(layout.code.bottom + 0.5);
  // The peek fits its region; its header is two compact rows, and the file name and line are one unbroken line.
  expect(layout.peek.bottom).toBeLessThanOrEqual(layout.main.bottom + 0.5);
  expect(layout.head.height).toBeLessThanOrEqual(76);
  expect(layout.file.height).toBeLessThanOrEqual(28);
  await expect(page.getByRole("region", { name: /^Source peek: identityaccess\/tenant\/tenant\.go:16$/ })).toBeVisible();
});

test("keeps a location's file name and line whole on the desktop inspector", async ({ page }) => {
  await openMap(page, "/");
  await selectOnMap(page, "identityaccess", "Aggregate Tenant");
  await assess(page);
  const location = evidence(page).getByRole("button", { name: /^Peek at identityaccess\/tenant\/tenant\.go:\d+$/ }).first();
  const file = location.locator(".st-path__file");
  await expect(file).toHaveText(/^tenant\.go:\d+$/);
  const box = await file.boundingBox();
  expect(box!.height).toBeLessThanOrEqual(28);
});
