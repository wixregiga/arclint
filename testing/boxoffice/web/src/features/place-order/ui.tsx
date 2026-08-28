// Strike the deal: name who the promise is for and place the order.
import { useMutation } from "@tanstack/react-query";
import { useState } from "react";

import { placeOrder, type Hold, type Order } from "../../shared/api";
import { ErrorNote } from "../../shared/ui";

export function PlaceOrderPanel({
  eventId,
  holds,
  onPlaced,
}: {
  eventId: string;
  holds: Hold[];
  onPlaced: (order: Order) => void;
}) {
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const place = useMutation({
    mutationFn: () =>
      placeOrder({ eventId, holdIds: holds.map((h) => h.holdId), attendee: { name, email } }),
    onSuccess: onPlaced,
  });

  if (holds.length === 0) return null;
  return (
    <section className="card held">
      <h3>Your held seats</h3>
      <ul>
        {holds.map((h) => (
          <li key={h.holdId}>
            {h.seats} × {h.tier}, held until {new Date(h.deadline).toLocaleTimeString()}
          </li>
        ))}
      </ul>
      <div className="actions">
        <input placeholder="name" value={name} onChange={(e) => setName(e.target.value)} />
        <input placeholder="email" value={email} onChange={(e) => setEmail(e.target.value)} />
        <button disabled={!name || !email || place.isPending} onClick={() => place.mutate()}>
          Place order
        </button>
      </div>
      <ErrorNote error={place.error} />
    </section>
  );
}
