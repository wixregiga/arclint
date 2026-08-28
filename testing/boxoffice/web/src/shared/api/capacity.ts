// The capacity ledger's Holds: seats set aside while someone is
// still deciding.
import { api } from "./client";

export interface Hold {
  holdId: string;
  tier: string;
  seats: number;
  deadline: string;
}

export function createHold(eventId: string, tier: string, seats: number): Promise<Hold> {
  return api.post(`/api/events/${eventId}/holds`, { tier, seats });
}
