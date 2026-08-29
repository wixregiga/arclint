// The organizer gives one buyer's tickets back, one ticket at a
// time. What the deal struck stays on the page next to what came
// back, because a placed order never changes.
import { useMutation, useQueryClient } from "@tanstack/react-query";

import {
  formatPrice,
  outstanding,
  refundTicket,
  type Order,
  type OrderLine,
} from "../../shared/api";
import { ErrorNote } from "../../shared/ui";

export function BuyerCard({ order, eventId }: { order: Order; eventId: string }) {
  const queryClient = useQueryClient();
  const refund = useMutation({
    mutationFn: (line: OrderLine) => refundTicket(order.id, line.tier, 1),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["organizer-event", eventId] });
      void queryClient.invalidateQueries({ queryKey: ["orders", order.id] });
      void queryClient.invalidateQueries({ queryKey: ["events"] });
    },
  });

  // Counted in tickets, not cents: a comped order owes nothing from
  // the start, so an empty balance never meant it came back.
  const fullyRefunded = order.lines.every((line) => outstanding(line) === 0);
  return (
    <li className="card">
      <div className="tier-row">
        <span className="tier-name">{order.attendee.name}</span>
        <span className="muted">{order.attendee.email}</span>
        {order.comped && <span className="chip">comped</span>}
        {fullyRefunded && <span className="chip cancelled">fully refunded</span>}
      </div>
      {order.lines.map((line) => (
        <div key={line.tier} className="tier-row">
          <span className="tier-name">{line.tier}</span>
          <span className="chip">{line.unitCents === 0 ? "free" : formatPrice(line.unitCents)}</span>
          <span className="muted">
            {outstanding(line)} of {line.quantity} held
            {line.refunded > 0 && `, ${line.refunded} refunded`}
          </span>
          <span className="stepper">
            <button
              disabled={outstanding(line) === 0 || refund.isPending}
              onClick={() => refund.mutate(line)}
            >
              Refund one
            </button>
          </span>
        </div>
      ))}
      <p className="muted">
        {order.comped ? (
          <>Comped, so nothing was ever owed</>
        ) : (
          <>
            Struck at {formatPrice(order.totalCents)} · still owed{" "}
            {formatPrice(order.outstandingCents)}
          </>
        )}
      </p>
      <ErrorNote error={refund.error} />
    </li>
  );
}
