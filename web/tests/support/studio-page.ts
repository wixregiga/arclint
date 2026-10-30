import { expect, type Locator, type Page } from "@playwright/test";

/** The Studio page as the suite drives it: the controls by their names. */
export class StudioPage {
  constructor(
    readonly page: Page,
    readonly base: string,
  ) {}

  async open(): Promise<void> {
    await this.page.goto(this.base);
    await expect(this.page.getByRole("navigation", { name: "The recorded language" })).toBeVisible();
  }

  get navigator(): Locator {
    return this.page.getByRole("navigation", { name: "The recorded language" });
  }

  get search(): Locator {
    return this.page.getByRole("combobox", { name: "Search the language" });
  }

  get card(): Locator {
    return this.page.locator("article.st-card").first();
  }

  async find(query: string, name: string): Promise<void> {
    await this.search.fill(query);
    await this.page.getByRole("option", { name: new RegExp(`^${escape(name)}\\b`) }).first().click();
  }

  /** Selects an entry by name in the navigator, unfolding its owner first when it is nested. */
  async select(name: string, owner?: string): Promise<void> {
    if (owner) {
      const fold = this.navigator.getByRole("button", { name: `Unfold ${owner}` });
      if (await fold.count()) await fold.click();
    }
    await this.navigator.getByRole("button", { name: new RegExp(`^(?!(Unfold|Fold) ).*\\b${escape(name)}( unsaved)?$`) }).first().click();
  }

  field(label: string): Locator {
    return this.card.getByLabel(new RegExp(`^${escape(label)}\\b`));
  }

  async overview(): Promise<OverviewLike> {
    const text = await this.page.evaluate(async (base) => {
      const r = await fetch(new URL("api/overview", base).toString(), { cache: "no-store" });
      return r.text();
    }, this.page.url());
    return JSON.parse(text) as OverviewLike;
  }


}

export interface OverviewLike {
  found: boolean;
  project?: string;
  description?: string;
  contexts?: {
    name: string;
    definition: string;
    aggregates?: { name: string; definition: string; identity: string; aliases?: string[]; entities?: { name: string }[]; invariants?: { key: string }[]; repository?: string }[];
    valueObjects?: { name: string; definition: string; aliases?: string[] }[];
    questions?: { key: string; text: string }[];
  }[];
}

function escape(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}
