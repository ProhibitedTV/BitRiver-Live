export type NavigationAudience = "guest" | "member" | "creator" | "admin";

export type NavigationItem = {
  label: string;
  href: string;
  visibleTo: readonly NavigationAudience[];
};

export type NavigationActor = {
  isAuthenticated: boolean;
  roles?: readonly string[];
};

const ALL_AUDIENCES: readonly NavigationAudience[] = ["guest", "member", "creator", "admin"];

export const CANONICAL_NAV_ITEMS: readonly NavigationItem[] = [
  { label: "Home", href: "/", visibleTo: ALL_AUDIENCES },
  { label: "Following", href: "/following", visibleTo: ALL_AUDIENCES },
  { label: "Browse", href: "/browse", visibleTo: ALL_AUDIENCES },
  { label: "Creator", href: "/creator", visibleTo: ["creator", "admin"] },
];

export const CANONICAL_QUICK_LINK_ITEMS: readonly NavigationItem[] = [
  { label: "Categories", href: "/browse", visibleTo: ALL_AUDIENCES },
  { label: "Following", href: "/following", visibleTo: ALL_AUDIENCES },
];

export function getNavigationAudience(actor: NavigationActor): NavigationAudience {
  if (!actor.isAuthenticated) {
    return "guest";
  }

  const roles = actor.roles ?? [];
  if (roles.includes("admin")) {
    return "admin";
  }
  if (roles.includes("creator")) {
    return "creator";
  }

  return "member";
}

export function getVisibleNavigationItems(
  audience: NavigationAudience,
  items: readonly NavigationItem[] = CANONICAL_NAV_ITEMS,
): NavigationItem[] {
  return items.filter((item) => item.visibleTo.includes(audience));
}

export function deriveQuickLinks(audience: NavigationAudience, navItems: readonly NavigationItem[]): NavigationItem[] {
  const navHrefs = new Set(navItems.map((item) => item.href));
  return getVisibleNavigationItems(audience, CANONICAL_QUICK_LINK_ITEMS).filter((item) => !navHrefs.has(item.href));
}
