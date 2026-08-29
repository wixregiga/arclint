// The deal as struck, the refunds recorded beside it, and the calls
// that place, read, and refund Orders.
import { api } from "./client";

export interface Attendee {
  name: string;
  email: string;
}

// OrderLine is one tier of the deal. quantity is what was bought and
// never moves; refunded is how many of those tickets came back.
export interface OrderLine {
  tier: string;
  quantity: number;
  refunded: number;
  unitCents: number;
}

// comped says the organizer gave this order away rather than an
// attendee buying it, so every ticket on it cost nothing.
export interface Order {
  id: string;
  eventId: string;
  attendee: Attendee;
  lines: OrderLine[];
  totalCents: number;
  outstandingCents: number;
  comped: boolean;
}

// outstanding is how many tickets of one line the buyer still holds.
export function outstanding(line: OrderLine): number {
  return line.quantity - line.refunded;
}

export interface PlaceOrder {
  eventId: string;
  holdIds: string[];
  attendee: Attendee;
}

export function placeOrder(input: PlaceOrder): Promise<Order> {
  return api.post("/api/orders", input);
}

export function getOrder(id: string): Promise<Order> {
  return api.get(`/api/orders/${id}`);
}

// refundTicket gives tickets back on one tier of one order. The deal
// as struck stays as struck; the refund is recorded beside it.
export function refundTicket(orderId: string, tier: string, quantity: number): Promise<Order> {
  return api.post(`/api/organizer/orders/${orderId}/refunds`, { tier, quantity });
}

// CompTicket is how many tickets of one tier the organizer is giving
// away. No price is sent, because a comped ticket is free.
export interface CompTicket {
  tier: string;
  quantity: number;
}

// compTickets gives tickets away on the organizer's say-so. The
// answer is an order like any other, costing its attendee nothing,
// with the seats already spoken for.
export function compTickets(
  eventId: string,
  attendee: Attendee,
  tickets: CompTicket[],
): Promise<Order> {
  return api.post(`/api/organizer/events/${eventId}/comps`, { attendee, tickets });
}
