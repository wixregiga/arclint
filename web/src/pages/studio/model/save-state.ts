import type { FeatureDocument } from "../../../shared/transport/index";

/**
 * Where the shown feature's last save stands. A failed or stale save
 * keeps every typed word in the draft; the state only says what to
 * offer the author next.
 */
export type FeatureSaveState =
  | { kind: "idle" }
  | { kind: "saving" }
  | { kind: "failed"; message: string; retry: (() => void) | null }
  /** The store holds a newer revision; `current` is that revision when it could be read. */
  | { kind: "stale"; current: FeatureDocument | null }
  /** The store no longer holds the feature; the draft can be saved as a new one. */
  | { kind: "missing" };

export const IDLE_SAVE: FeatureSaveState = { kind: "idle" };
