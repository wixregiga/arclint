import { useEffect, useReducer } from "react";
import type { StudioMemory } from "../../../shared/lib/studio-memory/index";
import { reduceWorkspace, restoreWorkspace, serializeWorkspace, type Workspace, type WorkspaceAction } from "./workspace";

/** The workspace, restored from memory on mount and written back on every change. */
export function useWorkspace(memory: StudioMemory): [Workspace, (action: WorkspaceAction) => void] {
  const [workspace, dispatch] = useReducer(reduceWorkspace, memory, (m) => restoreWorkspace(m.read()));
  useEffect(() => {
    memory.write(serializeWorkspace(workspace));
  }, [memory, workspace]);
  return [workspace, dispatch];
}
