"use client";

import { ReactNode, useEffect, useRef, useState } from "react";
import { usePathname } from "next/navigation";
import { FollowingSidebar } from "./FollowingSidebar";

interface ViewerShellProps {
  children: ReactNode;
}

const DESKTOP_SIDEBAR_QUERY = "(min-width: 1024px)";
const FOCUSABLE_SELECTORS =
  'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])';

function shouldShowFollowingSurface(pathname: string | null): boolean {
  const normalizedPath = pathname?.split("?")[0] || "/";

  return (
    normalizedPath === "/" ||
    normalizedPath === "/browse" ||
    normalizedPath.startsWith("/browse/") ||
    normalizedPath === "/videos" ||
    normalizedPath.startsWith("/videos/")
  );
}

export function ViewerShell({ children }: ViewerShellProps) {
  const pathname = usePathname();
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [desktopSidebar, setDesktopSidebar] = useState(false);
  const sidebarRef = useRef<HTMLElement | null>(null);
  const toggleButtonRef = useRef<HTMLButtonElement | null>(null);
  const previouslyFocusedRef = useRef<HTMLElement | null>(null);
  const followingSurfaceEnabled = shouldShowFollowingSurface(pathname);
  const persistentSidebar = desktopSidebar && followingSurfaceEnabled;
  const drawerAvailable = !desktopSidebar && followingSurfaceEnabled;

  const closeSidebar = () => {
    setSidebarOpen(false);
  };

  const toggleSidebar = () => {
    setSidebarOpen((open) => !open);
  };

  const modalSidebarOpen = drawerAvailable && sidebarOpen;

  useEffect(() => {
    if (typeof window === "undefined" || !window.matchMedia) {
      return;
    }

    const query = window.matchMedia(DESKTOP_SIDEBAR_QUERY);
    const syncDesktopState = (matches: boolean) => {
      setDesktopSidebar(matches);
      if (matches) {
        setSidebarOpen(false);
      }
    };

    syncDesktopState(query.matches);

    const handler = (event: MediaQueryListEvent) => {
      syncDesktopState(event.matches);
    };

    query.addEventListener("change", handler);
    return () => {
      query.removeEventListener("change", handler);
    };
  }, []);

  useEffect(() => {
    if (!followingSurfaceEnabled && sidebarOpen) {
      setSidebarOpen(false);
    }
  }, [followingSurfaceEnabled, sidebarOpen]);

  useEffect(() => {
    if (!modalSidebarOpen) {
      return;
    }

    previouslyFocusedRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const sidebarElement = sidebarRef.current;
    if (!sidebarElement) {
      return;
    }

    const focusables = Array.from(sidebarElement.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTORS)).filter(
      (element) => !element.hasAttribute("disabled"),
    );

    const target = focusables[0] ?? sidebarElement;
    target.focus();
  }, [modalSidebarOpen]);

  useEffect(() => {
    if (!modalSidebarOpen) {
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
        (element) => !element.hasAttribute("disabled"),
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
  }, [modalSidebarOpen]);

  useEffect(() => {
    if (!modalSidebarOpen) {
      return;
    }

    const priorOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";

    return () => {
      document.body.style.overflow = priorOverflow;
    };
  }, [modalSidebarOpen]);

  useEffect(() => {
    return () => {
      if (document.body.style.overflow === "hidden") {
        document.body.style.overflow = "";
      }
      previouslyFocusedRef.current = null;
    };
  }, []);

  return (
    <div
      className={`viewer-shell${modalSidebarOpen ? " viewer-shell--sidebar-open" : ""}${desktopSidebar ? " viewer-shell--desktop" : ""}${
        followingSurfaceEnabled ? " viewer-shell--following-enabled" : " viewer-shell--following-disabled"
      }${persistentSidebar ? " viewer-shell--following-persistent" : ""}`}
    >
      {followingSurfaceEnabled && (
        <aside
          id="viewer-sidebar"
          className="viewer-sidebar"
          aria-label={persistentSidebar ? "Following sidebar" : undefined}
          role={modalSidebarOpen ? "dialog" : undefined}
          aria-modal={modalSidebarOpen ? true : undefined}
          aria-labelledby={!persistentSidebar ? "viewer-sidebar-title" : undefined}
          ref={sidebarRef}
          tabIndex={-1}
        >
          {!persistentSidebar && (
            <div className="viewer-sidebar__header-row">
              <div className="stack stack--2xs">
                <p className="viewer-sidebar__eyebrow">Your network</p>
                <h2 id="viewer-sidebar-title">Following</h2>
              </div>
              <button type="button" className="viewer-sidebar__close" onClick={closeSidebar} aria-label="Close following sidebar">
                Close
              </button>
            </div>
          )}
          <p className="viewer-sidebar__intro muted">
            Keep your regular creators one click away while you explore live rooms, replays, and new communities.
          </p>
          <FollowingSidebar />
        </aside>
      )}

      {drawerAvailable && <div className="viewer-shell__backdrop" aria-hidden="true" onClick={closeSidebar} />}

      <div className="viewer-shell__content" aria-hidden={modalSidebarOpen}>
        <div className="viewer-shell__content-inner">
          {drawerAvailable && (
            <div className="viewer-shell__mobile-bar surface">
              <div className="stack stack--2xs">
                <span className="viewer-sidebar__eyebrow">Your network</span>
                <strong>Following</strong>
              </div>
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
          )}

          <main className="viewer-main">{children}</main>
          <footer className="footer">BitRiver Live helps independent creators go live, keep replays visible, and stay close to their audience.</footer>
        </div>
      </div>
    </div>
  );
}
