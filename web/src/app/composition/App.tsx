import { StrictMode } from "react";
import { StudioScreen } from "../../pages/studio/index";
import type { StudioMemory } from "../../shared/lib/studio-memory/index";
import type { CodeEvidenceSource, FeatureStore, StudioGateway } from "../../shared/transport/index";

export interface AppDependencies {
  gateway: StudioGateway;
  memory: StudioMemory;
  /** Where feature drafts are saved; null when the host lends none, and the Features view says so. */
  featureStore: FeatureStore | null;
  /** Read-only code evidence; null when the host lends none, and the inspector says so. */
  codeEvidence: CodeEvidenceSource | null;
}

/** Studio composed over the host's gateway, memory and feature store. */
export function App({ gateway, memory, featureStore, codeEvidence }: AppDependencies) {
  return (
    <StrictMode>
      <StudioScreen gateway={gateway} memory={memory} featureStore={featureStore} codeEvidence={codeEvidence} />
    </StrictMode>
  );
}
