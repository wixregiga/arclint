// One event's page: the story, the tiers, seats held while
// deciding, the deal struck.
import { useQuery } from "@tanstack/react-query";
import { useNavigate, useParams } from "@tanstack/react-router";
import { useState } from "react";

import { TierPicker } from "../../features/pick-tickets";
import { PlaceOrderPanel } from "../../features/place-order";
import { getEvent, type Hold } from "../../shared/api";
import { ErrorNote } from "../../shared/ui";

export function EventPage() {
  const { eventId = "" } = useParams({ strict: false });
  const navigate = useNavigate();
  const [holds, setHolds] = useState<Hold[]>([]);
  const event = useQuery({
    queryKey: ["events", eventId],
    queryFn: () => getEvent(eventId),
    enabled: eventId !== "",
  });

  if (event.error) return <ErrorNote error={event.error} />;
  if (!event.data) return <p>Loading</p>;
  const ev = event.data;
  return (
    <section>
      <div className="tier-row">
        <h2>{ev.title}</h2>
        {ev.status === "cancelled" && <span className="chip cancelled">cancelled</span>}
      </div>
      <p>{ev.story}</p>
      <p className="muted">
        {ev.when} · {ev.where}
      </p>
      {ev.status === "cancelled" ? (
        <p className="card">
          This show was called off. Every ticket sold for it has been refunded, and there is
          nothing left to buy.
        </p>
      ) : (
        <>
          <TierPicker event={ev} onHeld={(held) => setHolds((h) => [...h, ...held])} />
          <PlaceOrderPanel
            eventId={ev.id}
            holds={holds}
            onPlaced={(order) =>
              void navigate({ to: "/orders/$orderId", params: { orderId: order.id } })
            }
          />
        </>
      )}
    </section>
  );
}
