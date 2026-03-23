"use client";

import type { ReactNode } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";

type NavItem = {
  href: string;
  label: string;
};

function buildCreatorNav(pathname: string): NavItem[] {
  const items: NavItem[] = [
    { href: "/creator", label: "Overview" },
    { href: "/creator/getting-started", label: "Getting started" },
  ];

  const match = pathname.match(/^\/creator\/(live|uploads)\/([^/]+)/);
  if (!match) {
    return items;
  }

  const channelId = match[2];
  return [
    ...items,
    { href: `/creator/live/${channelId}`, label: "Go live" },
    { href: `/creator/uploads/${channelId}`, label: "Uploads" },
  ];
}

export default function CreatorLayout({ children }: { children: ReactNode }) {
  const pathname = usePathname() ?? "/creator";
  const navItems = buildCreatorNav(pathname);

  return (
    <div className="workspace-page workspace-page--narrow creator-layout">
      <header className="workspace-hero">
        <div className="workspace-hero__copy">
          <span className="page-eyebrow">Creator studio</span>
          <h1>Run your channel from one workspace</h1>
          <p className="muted">
            Move from onboarding to live setup to uploads without losing track of what to do next.
          </p>
        </div>
        <nav className="creator-layout__nav" aria-label="Creator navigation">
          {navItems.map((item) => {
            const isActive = pathname === item.href;
            return (
              <Link
                key={item.href}
                href={item.href}
                className={`creator-layout__nav-link${isActive ? " creator-layout__nav-link--active" : ""}`}
                aria-current={isActive ? "page" : undefined}
              >
                {item.label}
              </Link>
            );
          })}
        </nav>
        <div className="workspace-summary-grid">
          <article className="summary-card">
            <span className="summary-card__label">Broadcast</span>
            <strong className="summary-card__value">Guide</strong>
            <p className="muted">Check the setup flow, test stream health, and the public share link in one pass.</p>
          </article>
          <article className="summary-card">
            <span className="summary-card__label">Library</span>
            <strong className="summary-card__value">Monitor</strong>
            <p className="muted">Keep uploads, playback readiness, and recording follow-up in the same workspace.</p>
          </article>
          <article className="summary-card">
            <span className="summary-card__label">Clarity</span>
            <strong className="summary-card__value">Next step</strong>
            <p className="muted">Every screen now leads to the next creator action instead of ending in a dead end.</p>
          </article>
        </div>
      </header>
      <div className="workspace-shell">{children}</div>
    </div>
  );
}
