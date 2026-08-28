// Priya's side of the counter: her events, each opening its own
// page, and a way to start a new draft.
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { UnlockPanel } from "../../features/unlock-organizer";
import { createEvent, hasOrganizerToken, listAllEvents } from "../../shared/api";
import { ErrorNote } from "../../shared/ui";

export function OrganizerPage() {
  const [unlocked, setUnlocked] = useState(hasOrganizerToken);
  const [title, setTitle] = useState("");
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const events = useQuery({
    queryKey: ["organizer-events"],
    queryFn: listAllEvents,
    enabled: unlocked,
    retry: false,
  });
  const create = useMutation({
    mutationFn: () =>
      createEvent({
        title,
        story: "",
        when: "",
        where: "",
        tiers: [{ name: "general", priceCents: 0, seats: 50 }],
      }),
    onSuccess: (created) => {
      setTitle("");
      void queryClient.invalidateQueries({ queryKey: ["organizer-events"] });
      void navigate({ to: "/organizer/events/$eventId", params: { eventId: created.id } });
    },
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
      <h2>Your events</h2>
      <ErrorNote error={events.error} />
      <ul>
        {(events.data ?? []).map((ev) => (
          <li key={ev.id} className="card">
            <div className="tier-row">
              <h3>
                <Link to="/organizer/events/$eventId" params={{ eventId: ev.id }}>
                  {ev.title}
                </Link>
              </h3>
              <span className={`chip ${ev.status}`}>{ev.status}</span>
            </div>
          </li>
        ))}
      </ul>
      <div className="card">
        <h3>New draft</h3>
        <div className="actions">
          <input
            placeholder="new event title"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
          />
          <button disabled={!title || create.isPending} onClick={() => create.mutate()}>
            Create draft
          </button>
        </div>
        <ErrorNote error={create.error} />
      </div>
    </section>
  );
}
