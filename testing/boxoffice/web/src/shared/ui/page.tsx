// The web's shared shell and small kit pieces. The look itself
// lives in /styles.css, served next to the app.
import type { ReactNode } from "react";

export function PageShell({ title, children }: { title: string; children: ReactNode }) {
  return (
    <main className="shell">
      <header className="masthead">
        <h1>{title}</h1>
        <span className="muted">a small box office for real events</span>
      </header>
      {children}
    </main>
  );
}

export function ErrorNote({ error }: { error: unknown }) {
  if (!error) return null;
  const message = error instanceof Error ? error.message : String(error);
  return (
    <p role="alert" className="error-note">
      {message}
    </p>
  );
}
