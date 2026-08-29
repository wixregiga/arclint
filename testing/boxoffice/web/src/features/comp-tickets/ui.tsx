// The organizer gives tickets away: a name, a way to reach them, and
// how many on each tier. The seats are promised the moment the comp
// is given, so the room can refuse a comp it cannot fit.
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { compTickets, type CompTicket, type OrganizerTicketTier } from "../../shared/api";
import { ErrorNote } from "../../shared/ui";

export function CompPanel({ eventId, tiers }: { eventId: string; tiers: OrganizerTicketTier[] }) {
  const queryClient = useQueryClient();
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [counts, setCounts] = useState<Record<string, number>>({});

  const asked: CompTicket[] = tiers
    .map((tier) => ({ tier: tier.name, quantity: counts[tier.name] ?? 0 }))
    .filter((ticket) => ticket.quantity > 0);

  const comp = useMutation({
    mutationFn: () => compTickets(eventId, { name: name.trim(), email: email.trim() }, asked),
    onSuccess: () => {
      setName("");
      setEmail("");
      setCounts({});
      void queryClient.invalidateQueries({ queryKey: ["organizer-event", eventId] });
      void queryClient.invalidateQueries({ queryKey: ["events"] });
    },
  });

  const ready = name.trim() !== "" && email.trim() !== "" && asked.length > 0;
  return (
    <div className="card">
      <h3>Comp tickets</h3>
      <p className="muted">
        Give tickets to someone without charging them. The seats are promised the moment you comp
        them, so a comp cannot outrun the room, and the ticket comes back the same way any other
        does: refund it.
      </p>
      <div className="tier-row">
        <input
          aria-label="Who the tickets are for"
          placeholder="Who they are for"
          value={name}
          onChange={(event) => setName(event.target.value)}
        />
        <input
          aria-label="How to reach them"
          placeholder="How to reach them"
          value={email}
          onChange={(event) => setEmail(event.target.value)}
        />
      </div>
      {tiers.map((tier) => (
        <div key={tier.name} className="tier-row">
          <span className="tier-name">{tier.name}</span>
          <span className="muted">{tier.remaining} left</span>
          <span className="stepper">
            <input
              aria-label={`How many ${tier.name} tickets to comp`}
              type="number"
              min={0}
              max={tier.remaining}
              value={counts[tier.name] ?? 0}
              onChange={(event) =>
                setCounts((was) => ({ ...was, [tier.name]: Number(event.target.value) }))
              }
            />
          </span>
        </div>
      ))}
      <button disabled={!ready || comp.isPending} onClick={() => comp.mutate()}>
        Comp these tickets
      </button>
      <ErrorNote error={comp.error} />
    </div>
  );
}
