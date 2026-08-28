// The organizer edits a draft: the story, the when and where, and
// the ticket tiers with their prices and seats, saved all at once.
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { editEvent, type EventDetail } from "../../shared/api";
import { ErrorNote } from "../../shared/ui";
import { addTier, fromTiers, removeTier, setTier, toTiers, type TierDraft } from "./model";

export function DraftEditor({ event }: { event: EventDetail }) {
  const [story, setStory] = useState(event.story);
  const [when, setWhen] = useState(event.when);
  const [where, setWhere] = useState(event.where);
  const [tiers, setTiers] = useState<TierDraft[]>(() => fromTiers(event.tiers));
  const queryClient = useQueryClient();
  const save = useMutation({
    mutationFn: () => editEvent(event.id, { story, when, where, tiers: toTiers(tiers) }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["organizer-event", event.id] });
      void queryClient.invalidateQueries({ queryKey: ["organizer-events"] });
    },
  });

  return (
    <div className="card">
      <h3>The page</h3>
      <label className="field">
        <span className="muted">Story</span>
        <textarea
          value={story}
          placeholder="what this show is"
          onChange={(e) => setStory(e.target.value)}
        />
      </label>
      <label className="field">
        <span className="muted">When</span>
        <input
          value={when}
          placeholder="Friday 19:00"
          onChange={(e) => setWhen(e.target.value)}
        />
      </label>
      <label className="field">
        <span className="muted">Where</span>
        <input
          value={where}
          placeholder="The Garage Stage"
          onChange={(e) => setWhere(e.target.value)}
        />
      </label>
      <h3>Tickets</h3>
      {tiers.map((tier) => (
        <div key={tier.key} className="tier-row">
          <input
            className="grow"
            placeholder="tier name"
            value={tier.name}
            onChange={(e) => setTiers((t) => setTier(t, tier.key, { name: e.target.value }))}
          />
          <input
            className="compact"
            type="number"
            min="0"
            step="0.01"
            placeholder="price $"
            value={tier.price}
            onChange={(e) => setTiers((t) => setTier(t, tier.key, { price: e.target.value }))}
          />
          <input
            className="compact"
            type="number"
            min="1"
            step="1"
            placeholder="seats"
            value={tier.seats}
            onChange={(e) => setTiers((t) => setTier(t, tier.key, { seats: e.target.value }))}
          />
          <button
            className="step"
            title="remove this tier"
            onClick={() => setTiers((t) => removeTier(t, tier.key))}
          >
            −
          </button>
        </div>
      ))}
      <div className="actions">
        <button onClick={() => setTiers((t) => addTier(t))}>Add tier</button>
        <button disabled={save.isPending} onClick={() => save.mutate()}>
          Save draft
        </button>
        {save.isSuccess && !save.isPending && <span className="muted">Saved.</span>}
      </div>
      <ErrorNote error={save.error} />
    </div>
  );
}
