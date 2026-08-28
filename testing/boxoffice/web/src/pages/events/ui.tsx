// The front page: what's on sale, published events only.
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";

import { listEvents } from "../../shared/api";
import { ErrorNote } from "../../shared/ui";

export function EventsPage() {
  const events = useQuery({ queryKey: ["events"], queryFn: listEvents });
  return (
    <section>
      <h2>On sale</h2>
      <ErrorNote error={events.error} />
      <ul>
        {(events.data ?? []).map((ev) => (
          <li key={ev.id} className="card">
            <h3>
              <Link to="/events/$eventId" params={{ eventId: ev.id }}>
                {ev.title}
              </Link>
            </h3>
            <p className="muted">
              {ev.when} · {ev.where}
            </p>
          </li>
        ))}
      </ul>
    </section>
  );
}
