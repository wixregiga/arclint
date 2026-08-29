// The order page: what you bought, at the prices you paid.
import { useQuery } from "@tanstack/react-query";
import { useParams } from "@tanstack/react-router";

import { formatPrice, getOrder, outstanding } from "../../shared/api";
import { ErrorNote } from "../../shared/ui";

export function OrderPage() {
  const { orderId = "" } = useParams({ strict: false });
  const order = useQuery({
    queryKey: ["orders", orderId],
    queryFn: () => getOrder(orderId),
    enabled: orderId !== "",
  });

  if (order.error) return <ErrorNote error={order.error} />;
  if (!order.data) return <p>Loading</p>;
  const o = order.data;
  return (
    <section>
      <h2>Your order</h2>
      <div className="card">
        <p className="muted">
          {o.id} · for {o.attendee.name}
        </p>
        <ul>
          {o.lines.map((line) => (
            <li key={line.tier} className="tier-row">
              <span className="tier-name">
                {line.quantity} × {line.tier}
              </span>
              <span className="chip">
                {line.unitCents === 0 ? "free" : formatPrice(line.unitCents)}
              </span>
              {line.refunded > 0 && (
                <span className="chip cancelled">
                  {line.refunded} refunded, {outstanding(line)} left
                </span>
              )}
            </li>
          ))}
        </ul>
        <p className="total">
          {o.comped ? "These tickets are on the house" : `Total ${formatPrice(o.totalCents)}`}
        </p>
        {!o.comped && o.outstandingCents !== o.totalCents && (
          <p className="muted">
            Tickets came back on this order. What you bought stays here at the prices you paid;
            after the refunds, {formatPrice(o.outstandingCents)} of it still stands.
          </p>
        )}
      </div>
    </section>
  );
}
