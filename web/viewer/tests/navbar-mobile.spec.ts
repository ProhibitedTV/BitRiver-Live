import { expect, test } from "@playwright/test";

test.describe("navbar mobile layout", () => {
  test("collapses into a toggle on small viewports", async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 812 });
    await page.goto("/");

    const toggle = page.getByRole("button", { name: "Open navigation menu" });
    await expect(toggle).toBeVisible();

    const navMenu = page.locator("#viewer-nav-menu");
    await expect(navMenu).toBeHidden();

    await toggle.click();
    await expect(navMenu).toBeVisible();
    const browseLink = navMenu.getByRole("link", { name: "Browse", exact: true });
    await expect(browseLink).toBeVisible();

    const hasHorizontalOverflow = await page.evaluate(() => {
      return document.documentElement.scrollWidth > window.innerWidth;
    });
    expect(hasHorizontalOverflow).toBeFalsy();

    await browseLink.click();
    await expect(navMenu).toBeHidden();
  });

  test("keeps sidebar focus managed and supports escape/backdrop close on mobile", async ({ page }) => {
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
    await page.locator(".viewer-shell__backdrop").click();
    await expect(sidebarToggle).toBeFocused();
  });
});
