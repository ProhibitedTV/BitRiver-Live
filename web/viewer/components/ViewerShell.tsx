"use client";

import { ReactNode, useEffect, useState } from "react";
import { FollowingSidebar } from "./FollowingSidebar";

interface ViewerShellProps {
  children: ReactNode;
}

export function ViewerShell({ children }: ViewerShellProps) {
  const [sidebarOpen, setSidebarOpen] = useState(false);

  const toggleSidebar = () => setSidebarOpen((open) => !open);
  const closeSidebar = () => setSidebarOpen(false);

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        closeSidebar();
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, []);

  return (
    <div className={`viewer-shell ${sidebarOpen ? "viewer-shell--sidebar-open" : ""}`}>
      <aside id="viewer-sidebar" className="viewer-sidebar" aria-label="Following sidebar">
        <FollowingSidebar />
      </aside>

      <button
        type="button"
        aria-hidden={!sidebarOpen}
        className="viewer-shell__backdrop"
        onClick={closeSidebar}
        tabIndex={-1}
      />

      <div className="viewer-shell__content">
        <div className="viewer-shell__content-inner">
          <div className="viewer-shell__controls">
            <button
              type="button"
              className="viewer-shell__toggle"
              aria-expanded={sidebarOpen}
              aria-controls="viewer-sidebar"
              onClick={toggleSidebar}
            >
              {sidebarOpen ? "Hide following" : "Show following"}
            </button>
          </div>

          <main>{children}</main>
          <footer className="footer">Crafted for self-hosted creators · Powered by BitRiver Live</footer>
        </div>
      </div>
    </div>
  );
}
