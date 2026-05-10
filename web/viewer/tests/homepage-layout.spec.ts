import { expect, test } from "@playwright/test";

test.describe("homepage desktop shell", () => {
  test("keeps the following rail aligned with the featured stage", async ({ page }) => {
    await page.setViewportSize({ width: 1600, height: 1100 });
    await page.goto("/");

    const sidebar = page.locator(".viewer-sidebar");
    const hero = page.locator(".home-hero");

    await expect(sidebar).toBeVisible();
    await expect(hero).toBeVisible();
    await expect(page.locator(".viewer-shell")).toHaveClass(/viewer-shell--desktop/);
    await expect(page.getByText(/keep an eye on the creators you already know while you browse the rest of the platform/i)).toHaveCount(0);
    await expect(page.getByText("Sign in to see channels you follow.")).toHaveCount(1);

    const layout = await page.evaluate(() => {
      const top = (selector: string) => {
        const node = document.querySelector(selector);
        return node ? Math.round(node.getBoundingClientRect().top) : null;
      };

      return {
        sidebarTop: top(".viewer-sidebar"),
        heroTop: top(".home-hero"),
        sidebarHeaderTop: top(".following-sidebar__header"),
        heroHeaderTop: top(".home-hero__spotlight-header"),
      };
    });

    expect(layout.sidebarTop).not.toBeNull();
    expect(layout.heroTop).not.toBeNull();
    expect(layout.sidebarHeaderTop).not.toBeNull();
    expect(layout.heroHeaderTop).not.toBeNull();
    expect(Math.abs((layout.sidebarTop ?? 0) - (layout.heroTop ?? 0))).toBeLessThanOrEqual(8);
    expect(Math.abs((layout.sidebarHeaderTop ?? 0) - (layout.heroHeaderTop ?? 0))).toBeLessThanOrEqual(16);
  });
});
