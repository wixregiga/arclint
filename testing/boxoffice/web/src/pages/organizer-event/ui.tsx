// One event from the organizer's side of the counter: a draft is
// edited and published here; a published event shows the seats it
// published next to what has become of them, who bought tickets, and
// the way to give tickets back or call the show off.
import { useQuery } from "@tanstack/react-query";
import { Link, useParams } from "@tanstack/react-router";
import { useState } from "react";

import { CancelButton } from "../../features/cancel-event";
import { CompPanel } from "../../features/comp-tickets";
import { DraftEditor } from "../../features/edit-event";
import { PublishButton } from "../../features/publish-event";
import { BuyerCard } from "../../features/refund-ticket";
import { UnlockPanel } from "../../features/unlock-organizer";
import {
  formatPrice,
  getOrganizerEvent,
  hasOrganizerToken,
  type EventDetail,
  type OrganizerEventDetail,
} from "../../shared/api";
import { ErrorNote } from "../../shared/ui";

export function OrganizerEventPage() {
  const { eventId = "" } = useParams({ strict: false });
  const [unlocked, setUnlocked] = useState(hasOrganizerToken);
  const event = useQuery({
    queryKey: ["organizer-event", eventId],
    queryFn: () => getOrganizerEvent(eventId),
    enabled: unlocked && eventId !== "",
    retry: false,
  });

  if (!unlocked) {
    return (
      <section>
        <h2>Organizer</h2>
        <UnlockPanel onUnlocked={() => setUnlocked(true)} />
      </section>
    );
  }
  return (
    <section>
      <p>
        <Link to="/organizer">Back to your events</Link>
      </p>
      <ErrorNote error={event.error} />
      {!event.data && !event.error && <p>Loading</p>}
      {event.data && (
        <>
          <div className="tier-row">
            <h2>{event.data.title}</h2>
            <span className={`chip ${event.data.status}`}>{event.data.status}</span>
          </div>
          {event.data.status === "draft" ? (
            <>
              <DraftEditor key={event.data.id} event={asDraft(event.data)} />
              <div className="card">
                <h3>Publish</h3>
                <p className="muted">
                  Save the draft first; publishing needs at least one tier and a price on every
                  tier. Once published, the page cannot be edited.
                </p>
                <PublishButton eventId={event.data.id} />
              </div>
            </>
          ) : (
            <SoldDetails event={event.data} />
          )}
        </>
      )}
    </section>
  );
}

// asDraft narrows the organizer's view to what the draft editor
// needs: the page as it stands, with the seats left per tier.
function asDraft(event: OrganizerEventDetail): EventDetail {
  return {
    id: event.id,
    title: event.title,
    story: event.story,
    when: event.when,
    where: event.where,
    status: event.status,
    tiers: event.tiers.map((tier) => ({
      name: tier.name,
      priceCents: tier.priceCents,
      remaining: tier.remaining,
    })),
  };
}

// SoldDetails shows a published or cancelled event: nothing to edit,
// the seat counts as they stand, and the buyers.
function SoldDetails({ event }: { event: OrganizerEventDetail }) {
  const cancelled = event.status === "cancelled";
  const comped = event.orders.filter((order) => order.comped).length;
  return (
    <>
      <p>{event.story}</p>
      <p className="muted">
        {event.when} · {event.where}
      </p>
      <div className="card">
        <h3>Tickets</h3>
        <div className="tier-row counts counts-head">
          <span className="tier-name">tier</span>
          <span className="count-cell">published</span>
          <span className="count-cell">sold</span>
          <span className="count-cell">held</span>
          <span className="count-cell">left</span>
        </div>
        {event.tiers.map((tier) => (
          <div key={tier.name} className="tier-row counts">
            <span className="tier-name">
              {tier.name} <span className="chip">{formatPrice(tier.priceCents)}</span>
            </span>
            <span className="count-cell">{tier.seats}</span>
            <span className="count-cell">{tier.spokenFor}</span>
            <span className="count-cell">{tier.held}</span>
            <span className="count-cell">{tier.remaining}</span>
          </div>
        ))}
        <p className="muted">
          Published is the amount this tier went on sale with; it never moves. Sold is what is
          spoken for right now.
        </p>
      </div>

      {!cancelled && <CompPanel eventId={event.id} tiers={event.tiers} />}

      <h3>Who holds tickets</h3>
      {event.orders.length === 0 ? (
        <p className="muted">Nobody holds a ticket yet.</p>
      ) : (
        <>
          <p className="muted">
            Everyone promised a seat, whether they bought it or you comped it. {comped} of{" "}
            {event.orders.length} were comped.
          </p>
          <ul>
            {event.orders.map((order) => (
              <BuyerCard key={order.id} order={order} eventId={event.id} />
            ))}
          </ul>
        </>
      )}

      <div className="card">
        <h3>{cancelled ? "This show is off" : "Call the show off"}</h3>
        {cancelled ? (
          <p className="muted">
            The show was cancelled: it is off sale for good, every ticket sold was refunded, and
            the page cannot be edited or put back on sale.
          </p>
        ) : (
          <>
            <p className="muted">
              A published event is a promise already made; it cannot be edited. If the show cannot
              happen, cancelling it refunds every ticket at once and takes it off sale.
            </p>
            <CancelButton eventId={event.id} />
          </>
        )}
      </div>
    </>
  );
}
