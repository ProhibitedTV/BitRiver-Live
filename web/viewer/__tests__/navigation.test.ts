import {
  CANONICAL_NAV_ITEMS,
  CANONICAL_QUICK_LINK_ITEMS,
  deriveQuickLinks,
  getNavigationAudience,
  getVisibleNavigationItems,
} from "../lib/navigation";

describe("navigation config", () => {
  test.each([
    {
      actor: { isAuthenticated: false, roles: [] },
      expected: ["Home", "Following", "Browse"],
    },
    {
      actor: { isAuthenticated: true, roles: [] },
      expected: ["Home", "Following", "Browse"],
    },
    {
      actor: { isAuthenticated: true, roles: ["creator"] },
      expected: ["Home", "Following", "Browse", "Creator"],
    },
    {
      actor: { isAuthenticated: true, roles: ["admin"] },
      expected: ["Home", "Following", "Browse", "Creator"],
    },
  ])("resolves the expected nav labels for $actor", ({ actor, expected }) => {
    const audience = getNavigationAudience(actor);

    const labels = getVisibleNavigationItems(audience).map((item) => item.label);

    expect(labels).toEqual(expected);
  });

  test("derives quick links without duplicate hrefs already present in primary nav", () => {
    const audience = getNavigationAudience({ isAuthenticated: false, roles: [] });
    const navItems = getVisibleNavigationItems(audience);

    const quickLinks = deriveQuickLinks(audience, navItems);

    const navHrefs = new Set(navItems.map((item) => item.href));
    quickLinks.forEach((link) => {
      expect(navHrefs.has(link.href)).toBe(false);
    });
  });

  test("canonical config does not duplicate href entries within each list", () => {
    const navHrefs = CANONICAL_NAV_ITEMS.map((item) => item.href);
    const quickLinkHrefs = CANONICAL_QUICK_LINK_ITEMS.map((item) => item.href);

    expect(new Set(navHrefs).size).toBe(navHrefs.length);
    expect(new Set(quickLinkHrefs).size).toBe(quickLinkHrefs.length);
  });
});
