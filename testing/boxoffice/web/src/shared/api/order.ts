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

export interface Order {
  id: string;
  eventId: string;
  attendee: Attendee;
  lines: OrderLine[];
  totalCents: number;
  outstandingCents: number;
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
