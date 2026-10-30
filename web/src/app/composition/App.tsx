import { StrictMode } from "react";
import { StudioScreen } from "../../pages/studio/index";
import type { StudioMemory } from "../../shared/lib/studio-memory/index";
import type { StudioGateway } from "../../shared/transport/index";

export interface AppDependencies {
  gateway: StudioGateway;
  memory: StudioMemory;
}

/** Studio composed over the host's gateway and memory. */
export function App({ gateway, memory }: AppDependencies) {
  return (
    <StrictMode>
      <StudioScreen gateway={gateway} memory={memory} />
    </StrictMode>
  );
}
