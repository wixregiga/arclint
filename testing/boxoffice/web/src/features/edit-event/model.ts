// The editor's working copy of a draft's tier list: strings while
// the organizer types, converted to the api shape on save.
import type { NewTier, TicketTier } from "../../shared/api";

export interface TierDraft {
  key: string;
  name: string;
  // Price in dollars as typed, like "22.00"; empty means no price yet.
  price: string;
  // Seats the room gives this tier, as typed.
  seats: string;
}

let mintCount = 0;

function mintKey(): string {
  mintCount += 1;
  return `tier-${mintCount}`;
}

// fromTiers seeds the working copy from the draft as stored. A draft
// has no holds or orders, so each tier's remaining count is exactly
// the seats it was given.
export function fromTiers(tiers: TicketTier[]): TierDraft[] {
  return tiers.map((t) => ({
    key: mintKey(),
    name: t.name,
    price: t.priceCents > 0 ? (t.priceCents / 100).toFixed(2) : "",
    seats: String(t.remaining),
  }));
}

export function setTier(list: TierDraft[], key: string, patch: Partial<TierDraft>): TierDraft[] {
  return list.map((t) => (t.key === key ? { ...t, ...patch, key: t.key } : t));
}

export function addTier(list: TierDraft[]): TierDraft[] {
  return [...list, { key: mintKey(), name: "", price: "", seats: "" }];
}

export function removeTier(list: TierDraft[], key: string): TierDraft[] {
  return list.filter((t) => t.key !== key);
}

function num(text: string): number {
  const n = Number(text);
  return Number.isFinite(n) ? n : 0;
}

// toTiers converts the working copy to the api shape: dollars become
// whole cents, and blanks become zero so the server can refuse them
// with its own words.
export function toTiers(list: TierDraft[]): NewTier[] {
  return list.map((t) => ({
    name: t.name.trim(),
    priceCents: Math.round(num(t.price) * 100),
    seats: Math.trunc(num(t.seats)),
  }));
}
