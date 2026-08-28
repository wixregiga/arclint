// The catalog's Event as the web sees it, and every call that
// speaks about events. One file per recorded aggregate lives in
// shared/api; the arclint expansion rule derives that requirement
// from the vocabulary.
import { api } from "./client";
import type { Order } from "./order";

export interface TicketTier {
  name: string;
  priceCents: number;
  remaining: number;
}

// EventStatus is where an event stands: a draft nobody but its
// organizer sees, published and on sale, or cancelled and off sale
// for good.
export type EventStatus = "draft" | "published" | "cancelled";

export interface EventDetail {
  id: string;
  title: string;
  story: string;
  when: string;
  where: string;
  status: EventStatus;
  tiers: TicketTier[];
}

// OrganizerTicketTier is a tier as its organizer needs to see it:
// the seats published, next to what has become of them.
export interface OrganizerTicketTier {
  name: string;
  priceCents: number;
  seats: number;
  spokenFor: number;
  held: number;
  remaining: number;
}

// OrganizerEventDetail is one event from behind the counter: the
// page as it stands, the seat counts per tier, and who bought
// tickets.
export interface OrganizerEventDetail {
  id: string;
  title: string;
  story: string;
  when: string;
  where: string;
  status: EventStatus;
  tiers: OrganizerTicketTier[];
  orders: Order[];
}

export interface NewTier {
  name: string;
  priceCents: number;
  seats: number;
}

export interface NewEvent {
  title: string;
  story: string;
  when: string;
  where: string;
  tiers: NewTier[];
}

export function listEvents(): Promise<EventDetail[]> {
  return api.get("/api/events");
}

export function getEvent(id: string): Promise<EventDetail> {
  return api.get(`/api/events/${id}`);
}

export function listAllEvents(): Promise<EventDetail[]> {
  return api.get("/api/organizer/events");
}

// getOrganizerEvent reads one event from behind the counter, with the
// published seat amounts and the buyers the public page never shows.
export function getOrganizerEvent(id: string): Promise<OrganizerEventDetail> {
  return api.get(`/api/organizer/events/${id}`);
}

export function createEvent(input: NewEvent): Promise<EventDetail> {
  return api.post("/api/organizer/events", input);
}

// EditEvent is a draft as the organizer's editor saves it: the whole
// page at once. Tiers replace the draft's list wholesale.
export interface EditEvent {
  story: string;
  when: string;
  where: string;
  tiers: NewTier[];
}

export function editEvent(id: string, input: EditEvent): Promise<EventDetail> {
  return api.put(`/api/organizer/events/${id}`, input);
}

export function publishEvent(id: string): Promise<EventDetail> {
  return api.post(`/api/organizer/events/${id}/publish`);
}

// cancelEvent calls a published show off: it goes off sale for good
// and every ticket sold for it is given back.
export function cancelEvent(id: string): Promise<EventDetail> {
  return api.post(`/api/organizer/events/${id}/cancel`);
}

export function formatPrice(cents: number): string {
  return `$${(cents / 100).toFixed(2)}`;
}
