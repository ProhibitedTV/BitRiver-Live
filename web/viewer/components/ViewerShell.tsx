"use client";

import { ReactNode, useEffect, useRef, useState } from "react";
import { FollowingSidebar } from "./FollowingSidebar";

interface ViewerShellProps {
  children: ReactNode;
}

const FOCUSABLE_SELECTORS =
  'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])';

export function ViewerShell({ children }: ViewerShellProps) {
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const sidebarRef = useRef<HTMLElement | null>(null);
  const toggleButtonRef = useRef<HTMLButtonElement | null>(null);
  const previouslyFocusedRef = useRef<HTMLElement | null>(null);

  const closeSidebar = () => {
    setSidebarOpen(false);
  };

  const toggleSidebar = () => {
    setSidebarOpen((open) => !open);
  };

  useEffect(() => {
    if (!sidebarOpen) {
      return;
    }

    previouslyFocusedRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const sidebarElement = sidebarRef.current;
    if (!sidebarElement) {
      return;
    }

    const focusables = Array.from(sidebarElement.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTORS)).filter(
      (element) => !element.hasAttribute("disabled")
    );

    const target = focusables[0] ?? sidebarElement;
    target.focus();
  }, [sidebarOpen]);

  useEffect(() => {
    if (!sidebarOpen) {
      const focusTarget = toggleButtonRef.current ?? previouslyFocusedRef.current;
      focusTarget?.focus();
      return;
    }

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        closeSidebar();
        return;
      }

      if (event.key !== "Tab") {
        return;
      }

      const sidebarElement = sidebarRef.current;
      if (!sidebarElement) {
        return;
      }

      const focusables = Array.from(sidebarElement.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTORS)).filter(
        (element) => !element.hasAttribute("disabled")
      );

      if (focusables.length === 0) {
        event.preventDefault();
        sidebarElement.focus();
        return;
      }

      const activeElement = document.activeElement instanceof HTMLElement ? document.activeElement : null;
      const currentIndex = focusables.indexOf(activeElement ?? focusables[0]);

      if (event.shiftKey) {
        if (currentIndex <= 0) {
          event.preventDefault();
          focusables[focusables.length - 1]?.focus();
        }
        return;
      }

      if (currentIndex === -1 || currentIndex >= focusables.length - 1) {
        event.preventDefault();
        focusables[0]?.focus();
      }
    };

    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [sidebarOpen]);

  useEffect(() => {
    if (!sidebarOpen) {
      return;
    }

    const priorOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";

    return () => {
      document.body.style.overflow = priorOverflow;
    };
  }, [sidebarOpen]);

  useEffect(() => {
    return () => {
      if (document.body.style.overflow === "hidden") {
        document.body.style.overflow = "";
      }
      previouslyFocusedRef.current = null;
    };
  }, []);

  return (
    <div className={`viewer-shell ${sidebarOpen ? "viewer-shell--sidebar-open" : ""}`}>
      <aside
        id="viewer-sidebar"
        className="viewer-sidebar"
        aria-label="Following sidebar"
        role="dialog"
        aria-modal={sidebarOpen ? true : undefined}
        aria-labelledby="viewer-sidebar-title"
        ref={sidebarRef}
        tabIndex={-1}
      >
        <div className="viewer-sidebar__header-row">
          <h2 id="viewer-sidebar-title">Following</h2>
          <button type="button" className="viewer-sidebar__close" onClick={closeSidebar} aria-label="Close following sidebar">
            Close
          </button>
        </div>
        <FollowingSidebar />
      </aside>

      <div className="viewer-shell__backdrop" aria-hidden="true" onClick={closeSidebar} />

      <div className="viewer-shell__content" aria-hidden={sidebarOpen}>
        <div className="viewer-shell__content-inner">
          <div className="viewer-shell__controls">
            <button
              type="button"
              className="viewer-shell__toggle"
              aria-expanded={sidebarOpen}
              aria-controls="viewer-sidebar"
              onClick={toggleSidebar}
              ref={toggleButtonRef}
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
