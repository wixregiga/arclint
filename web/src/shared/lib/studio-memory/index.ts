/**
 * Where Studio keeps what the page holds between visits: the drafts not yet
 * saved, the selection and the mode. A host injects one; the standalone
 * page uses the browser's session storage under its own key, and an
 * embedded mount that injects none keeps it in memory for its lifetime,
 * so mounting Studio never writes to the host's storage.
 */
export interface StudioMemory {
  read(): string | null;
  write(text: string): void;
  forget(): void;
}

export function inMemoryStudioMemory(): StudioMemory {
  let held: string | null = null;
  return {
    read: () => held,
    write: (text) => {
      held = text;
    },
    forget: () => {
      held = null;
    },
  };
}

/** Memory in a Web Storage area under one key; a storage that refuses (quota, privacy mode) is treated as empty. */
export function storageStudioMemory(storage: Storage, key: string): StudioMemory {
  return {
    read: () => {
      try {
        return storage.getItem(key);
      } catch {
        return null;
      }
    },
    write: (text) => {
      try {
        storage.setItem(key, text);
      } catch {
        // Nothing to do: the page keeps its state in memory until it is left.
      }
    },
    forget: () => {
      try {
        storage.removeItem(key);
      } catch {
        // Nothing to do.
      }
    },
  };
}
