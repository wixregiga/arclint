import { entryKey, ownerOf, type Entry } from "../../../entities/language/index";
import type { Selection } from "../model/workspace";

export function contextSelection(context: string): Selection {
  return { kind: "entry", key: entryKey(context, "bounded_context", "", context) };
}

/** Where to stand once an entry is gone: its owner, else its context, else the project. */
export function afterRemoval(entries: readonly Entry[], removed: Entry): Selection {
  if (removed.concept === "bounded_context") return { kind: "project" };
  const owner = ownerOf(entries, removed);
  if (owner) return { kind: "entry", key: owner.key };
  return contextSelection(removed.context);
}

/** Where a cancelled new entry returns to: its owner when nested, else its context, else the project. */
export function whereBegun(entries: readonly Entry[], context: string, owner: string): Selection {
  if (owner !== "") {
    const o = entries.find((e) => e.context === context && e.name === owner && e.owner === "");
    if (o) return { kind: "entry", key: o.key };
  }
  if (context !== "") return contextSelection(context);
  return { kind: "project" };
}
