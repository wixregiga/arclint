// Pick tiers and quantities, then hold the seats while deciding.
import { useMutation } from "@tanstack/react-query";
import { useState } from "react";

import { createHold, formatPrice, type EventDetail, type Hold } from "../../shared/api";
import { ErrorNote } from "../../shared/ui";
import { adjust, picked, type Selection } from "./model";

export function TierPicker({
  event,
  onHeld,
}: {
  event: EventDetail;
  onHeld: (holds: Hold[]) => void;
}) {
  const [selection, setSelection] = useState<Selection>({});
  const hold = useMutation({
    mutationFn: async () => {
      const holds: Hold[] = [];
      for (const [tier, seats] of picked(selection)) {
        holds.push(await createHold(event.id, tier, seats));
      }
      return holds;
    },
    onSuccess: (holds) => {
      setSelection({});
      onHeld(holds);
    },
  });

  return (
    <section className="card">
      <h3>Tickets</h3>
      {event.tiers.map((tier) => (
        <div key={tier.name} className="tier-row">
          <span className="tier-name">{tier.name}</span>
          <span className="chip">{formatPrice(tier.priceCents)}</span>
          {tier.remaining === 0 ? (
            <span className="chip soldout">sold out</span>
          ) : (
            <span className="muted">{tier.remaining} left</span>
          )}
          <span className="stepper">
            <button className="step" onClick={() => setSelection((s) => adjust(s, tier.name, -1))}>
              −
            </button>
            <span className="count">{selection[tier.name] ?? 0}</span>
            <button
              className="step"
              disabled={tier.remaining === 0}
              onClick={() => setSelection((s) => adjust(s, tier.name, +1))}
            >
              +
            </button>
          </span>
        </div>
      ))}
      <div className="actions">
        <button
          disabled={picked(selection).length === 0 || hold.isPending}
          onClick={() => hold.mutate()}
        >
          Hold seats
        </button>
      </div>
      <ErrorNote error={hold.error} />
    </section>
  );
}
