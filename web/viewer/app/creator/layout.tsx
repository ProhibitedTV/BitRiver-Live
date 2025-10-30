import type { ReactNode } from "react";

export default function CreatorLayout({ children }: { children: ReactNode }) {
  return (
    <div className="container" style={{ paddingTop: "2rem", paddingBottom: "4rem" }}>
      <div className="stack" style={{ gap: "2rem" }}>
        <header className="stack">
          <h1>Creator dashboard</h1>
          <p className="muted">Manage channel uploads, schedules, and community tools.</p>
        </header>
        {children}
      </div>
    </div>
  );
}
