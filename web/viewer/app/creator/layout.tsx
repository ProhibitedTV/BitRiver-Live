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
      <header className="creator-layout__bar">
        <div className="creator-layout__heading">
          <span className="page-eyebrow">Creator</span>
          <strong>Channel workspace</strong>
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
      </header>
      <div className="workspace-shell">{children}</div>
    </div>
  );
}
