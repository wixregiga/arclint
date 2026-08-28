// The organizer unlocks their side of the counter with the token.
import { useState } from "react";

import { setOrganizerToken } from "../../shared/api";

export function UnlockPanel({ onUnlocked }: { onUnlocked: () => void }) {
  const [token, setToken] = useState("");
  return (
    <div className="card">
      <p className="muted">This side of the counter is the organizer's alone.</p>
      <div className="actions">
        <input
          placeholder="organizer token"
          value={token}
          onChange={(e) => setToken(e.target.value)}
        />
        <button
          disabled={!token}
          onClick={() => {
            setOrganizerToken(token);
            onUnlocked();
          }}
        >
          Unlock
        </button>
      </div>
    </div>
  );
}
