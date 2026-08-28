// The organizer flips a draft to published, once it is ready.
import { useMutation, useQueryClient } from "@tanstack/react-query";

import { publishEvent } from "../../shared/api";
import { ErrorNote } from "../../shared/ui";

export function PublishButton({ eventId }: { eventId: string }) {
  const queryClient = useQueryClient();
  const publish = useMutation({
    mutationFn: () => publishEvent(eventId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["organizer-events"] });
      void queryClient.invalidateQueries({ queryKey: ["organizer-event", eventId] });
    },
  });
  return (
    <span>
      <button disabled={publish.isPending} onClick={() => publish.mutate()}>
        Publish
      </button>
      <ErrorNote error={publish.error} />
    </span>
  );
}
