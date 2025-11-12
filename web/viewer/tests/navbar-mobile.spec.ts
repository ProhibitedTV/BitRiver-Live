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
    await expect(page.getByRole("link", { name: "Directory" })).toBeVisible();

    const hasHorizontalOverflow = await page.evaluate(() => {
      return document.documentElement.scrollWidth > window.innerWidth;
    });
    expect(hasHorizontalOverflow).toBeFalsy();

    await page.getByRole("link", { name: "Directory" }).click();
    await expect(navMenu).toBeHidden();
  });
});
