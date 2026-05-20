import { expect, test, type Page } from "@playwright/test";

const followedChannel = {
  channel: {
    id: "nav-followed",
    ownerId: "owner-nav-followed",
    title: "Navigation Followed Channel",
    category: "Music",
    tags: [],
    liveState: "Live",
    createdAt: new Date("2026-01-01T00:00:00Z").toISOString(),
    updatedAt: new Date("2026-01-01T00:00:00Z").toISOString(),
  },
  owner: { id: "owner-nav-followed", displayName: "Nav Creator" },
  profile: {},
  live: true,
  followerCount: 12,
};

async function mockPopulatedFollowing(page: Page) {
  await page.route("**/api/viewer/me", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        user: {
          id: "viewer-1",
          displayName: "Viewer",
          email: "viewer@example.com",
          roles: ["member"],
        },
      }),
    });
  });

  await page.route("**/api/directory/following", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ channels: [followedChannel], generatedAt: new Date("2026-01-01T00:00:00Z").toISOString() }),
    });
  });
}

test.describe("navbar mobile layout", () => {
  test("collapses into a toggle on small viewports", async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 812 });
    await page.goto("/");

    const toggle = page.getByRole("button", { name: "Open navigation menu" });
    await expect(toggle).toBeVisible();
    await expect(page.locator(".nav-tabs")).toBeHidden();

    const navMenu = page.locator("#viewer-nav-menu");
    await expect(navMenu).toBeHidden();

    await toggle.click();
    await expect(navMenu).toBeVisible();
    await expect(navMenu.getByRole("link", { name: "Browse", exact: true })).toBeVisible();
    await expect(navMenu.getByRole("link", { name: "Creator setup", exact: true })).toBeVisible();

    const hasHorizontalOverflow = await page.evaluate(() => {
      return document.documentElement.scrollWidth > window.innerWidth;
    });
    expect(hasHorizontalOverflow).toBeFalsy();

    await navMenu.getByRole("link", { name: "Browse", exact: true }).click();
    await expect(navMenu).toBeHidden();
  });

  test("keeps sidebar focus managed and supports escape/backdrop close on mobile", async ({ page }) => {
    await mockPopulatedFollowing(page);
    await page.setViewportSize({ width: 375, height: 812 });
    await page.goto("/");

    const sidebarToggle = page.getByRole("button", { name: "Show following" });
    await expect(sidebarToggle).toBeVisible();
    await sidebarToggle.click();

    const closeSidebar = page.getByRole("button", { name: "Close following sidebar" });
    await expect(closeSidebar).toBeVisible();
    await expect(closeSidebar).toBeFocused();

    await page.keyboard.press("Shift+Tab");
    const focusInsideSidebar = await page.evaluate(() => {
      const sidebar = document.getElementById("viewer-sidebar");
      const active = document.activeElement;
      return Boolean(sidebar && active && sidebar.contains(active));
    });
    expect(focusInsideSidebar).toBeTruthy();

    await page.keyboard.press("Tab");
    await expect(closeSidebar).toBeFocused();

    await page.keyboard.press("Escape");
    await expect(sidebarToggle).toBeFocused();

    await sidebarToggle.click();
    const backdropBox = await page.locator(".viewer-shell__backdrop").boundingBox();
    expect(backdropBox).not.toBeNull();
    await page.mouse.click(backdropBox!.x + backdropBox!.width - 6, backdropBox!.y + backdropBox!.height - 6);
    await expect(sidebarToggle).toBeFocused();
  });
});
