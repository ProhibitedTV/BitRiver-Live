"use client";

import { ReactNode, useEffect, useRef, useState } from "react";
import { usePathname } from "next/navigation";
import { FollowingSidebarContent } from "./FollowingSidebar";
import { useFollowingChannels } from "./following/useFollowingChannels";
import { useAuth } from "../hooks/useAuth";

interface ViewerShellProps {
  children: ReactNode;
}

const FOLLOWING_REFRESH_INTERVAL_MS = 30_000;
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
  const followingSurfaceEnabled = shouldShowFollowingSurface(pathname);
  const { user, loading: authLoading } = useAuth();
  const { channels: followingChannels, status: followingStatus, reload: reloadFollowing } = useFollowingChannels({
    isAuthenticated: followingSurfaceEnabled && Boolean(user),
    authLoading,
    refreshIntervalMs: followingSurfaceEnabled ? FOLLOWING_REFRESH_INTERVAL_MS : undefined,
  });
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const sidebarRef = useRef<HTMLElement | null>(null);
  const toggleButtonRef = useRef<HTMLButtonElement | null>(null);
  const previouslyFocusedRef = useRef<HTMLElement | null>(null);
  const followingChromeVisible = followingSurfaceEnabled && followingStatus === "ready" && followingChannels.length > 0;
  const drawerAvailable = followingChromeVisible;
  const followedCountLabel = `${followingChannels.length} followed`;

  const closeSidebar = () => {
    setSidebarOpen(false);
  };

  const toggleSidebar = () => {
    setSidebarOpen((open) => !open);
  };

  const modalSidebarOpen = drawerAvailable && sidebarOpen;

  useEffect(() => {
    if (!followingChromeVisible && sidebarOpen) {
      setSidebarOpen(false);
    }
  }, [followingChromeVisible, sidebarOpen]);

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
      className={`viewer-shell${modalSidebarOpen ? " viewer-shell--sidebar-open" : ""}${
        followingChromeVisible ? " viewer-shell--following-enabled" : " viewer-shell--following-disabled"
      }`}
    >
      {followingChromeVisible && (
        <aside
          id="viewer-sidebar"
          className="viewer-sidebar"
          role={modalSidebarOpen ? "dialog" : undefined}
          aria-modal={modalSidebarOpen ? true : undefined}
          aria-hidden={modalSidebarOpen ? undefined : true}
          aria-labelledby="viewer-sidebar-title"
          ref={sidebarRef}
          tabIndex={-1}
        >
          <div className="viewer-sidebar__header-row">
            <div className="stack stack--2xs">
              <p className="viewer-sidebar__eyebrow">Following</p>
              <h2 id="viewer-sidebar-title">Followed channels</h2>
            </div>
            <button type="button" className="viewer-sidebar__close" onClick={closeSidebar} aria-label="Close following sidebar">
              Close
            </button>
          </div>
          <FollowingSidebarContent
            channels={followingChannels}
            status={followingStatus}
            onRetry={() => {
              void reloadFollowing();
            }}
          />
        </aside>
      )}

      {drawerAvailable && <div className="viewer-shell__backdrop" aria-hidden="true" onClick={closeSidebar} />}

      <div className="viewer-shell__content" aria-hidden={modalSidebarOpen}>
        <div className="viewer-shell__content-inner">
          {drawerAvailable && (
            <div className="viewer-shell__mobile-bar surface">
              <div className="stack stack--2xs">
                <span className="viewer-sidebar__eyebrow">Following</span>
                <strong>{followedCountLabel}</strong>
              </div>
              <button
                type="button"
                className="viewer-shell__toggle"
                aria-expanded={sidebarOpen}
                aria-label={sidebarOpen ? "Hide following" : "Show following"}
                aria-controls="viewer-sidebar"
                onClick={toggleSidebar}
                ref={toggleButtonRef}
              >
                {sidebarOpen ? "Hide" : "Open"}
              </button>
            </div>
          )}

          <main className="viewer-main">{children}</main>
          <footer className="footer">BitRiver Live: live channels, replays, and creator tools.</footer>
        </div>
      </div>
    </div>
  );
}
