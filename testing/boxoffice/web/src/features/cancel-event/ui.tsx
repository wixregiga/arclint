// The organizer calls a published show off. It is final and it
// refunds every ticket sold, so the button asks once before it does.
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { cancelEvent } from "../../shared/api";
import { ErrorNote } from "../../shared/ui";

export function CancelButton({ eventId }: { eventId: string }) {
  const [asking, setAsking] = useState(false);
  const queryClient = useQueryClient();
  const cancel = useMutation({
    mutationFn: () => cancelEvent(eventId),
    onSuccess: () => {
      setAsking(false);
      void queryClient.invalidateQueries({ queryKey: ["organizer-events"] });
      void queryClient.invalidateQueries({ queryKey: ["organizer-event", eventId] });
      void queryClient.invalidateQueries({ queryKey: ["events"] });
    },
  });

  return (
    <div>
      {asking ? (
        <div className="actions">
          <span className="muted">
            Cancelling is final: the show comes off sale for good and every ticket sold is
            refunded.
          </span>
          <button disabled={cancel.isPending} onClick={() => cancel.mutate()}>
            Yes, cancel the show
          </button>
          <button className="step" title="keep the show on sale" onClick={() => setAsking(false)}>
            ×
          </button>
        </div>
      ) : (
        <div className="actions">
          <button onClick={() => setAsking(true)}>Cancel the show</button>
        </div>
      )}
      <ErrorNote error={cancel.error} />
    </div>
  );
}
