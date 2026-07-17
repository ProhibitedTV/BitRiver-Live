import { expect, test, type Page } from "@playwright/test";

const followedChannel = {
  channel: {
    id: "chan-followed",
    ownerId: "owner-followed",
    title: "Synth Garden",
    category: "Music",
    tags: ["ambient"],
    liveState: "Live",
    createdAt: new Date("2026-01-01T00:00:00Z").toISOString(),
    updatedAt: new Date("2026-01-01T00:00:00Z").toISOString(),
  },
  owner: {
    id: "owner-followed",
    displayName: "Ari Wave",
  },
  profile: {},
  live: true,
  followerCount: 24,
};

async function mockViewerSession(page: Page, signedIn: boolean) {
  await page.route("**/api/viewer/me", async (route) => {
    if (!signedIn) {
      await route.fulfill({
        status: 401,
        contentType: "application/json",
        body: JSON.stringify({ allowSelfSignup: true }),
      });
      return;
    }

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
}

async function mockFollowing(page: Page, channels = [followedChannel]) {
  await page.route("**/api/directory/following", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ channels, generatedAt: new Date("2026-01-01T00:00:00Z").toISOString() }),
    });
  });
}

test.describe("homepage desktop shell", () => {
  test("keeps guest following state out of discovery chrome", async ({ page }) => {
    await mockViewerSession(page, false);
    await page.setViewportSize({ width: 1600, height: 1100 });
    await page.goto("/");

    const sidebar = page.locator(".viewer-sidebar");
    const heroes = page.locator(".home-hero");
    const settledHero = page.locator(".home-hero:visible").filter({ hasText: /Relay ready|Signal live/ });

    await expect(settledHero).toBeVisible();
    await expect(heroes).toHaveCount(1);
    await expect(sidebar).toHaveCount(0);
    await expect(page.locator(".viewer-shell")).toHaveClass(/viewer-shell--following-disabled/);
    await expect(page.getByText(/keep an eye on the creators you already know while you browse the rest of the platform/i)).toHaveCount(0);
    await expect(page.getByText("Sign in to see channels you follow.")).toHaveCount(0);
  });

  test("keeps followed channels behind an intentional desktop drawer", async ({ page }) => {
    await mockViewerSession(page, true);
    await mockFollowing(page);
    await page.setViewportSize({ width: 1440, height: 1000 });

    await page.goto("/");

    await expect(page.locator(".viewer-shell")).toHaveClass(/viewer-shell--following-enabled/);
    await expect(page.locator(".viewer-shell")).not.toHaveClass(/viewer-shell--following-persistent/);
    await expect(page.locator(".viewer-shell")).not.toHaveClass(/viewer-shell--sidebar-open/);
    await expect(page.locator(".viewer-shell__mobile-bar").getByText("1 followed")).toBeVisible();
    await expect(page.locator(".viewer-sidebar")).toHaveCSS("opacity", "0");

    await page.getByRole("button", { name: "Show following" }).click();

    await expect(page.locator(".viewer-shell")).toHaveClass(/viewer-shell--sidebar-open/);
    await expect(page.locator(".viewer-sidebar")).toHaveCSS("opacity", "1");
    await expect(page.locator(".viewer-sidebar").getByRole("link", { name: /ari wave/i })).toBeVisible();
  });

  test("keeps empty following out of the mobile drawer chrome", async ({ page }) => {
    await mockViewerSession(page, true);
    await mockFollowing(page, []);
    await page.setViewportSize({ width: 390, height: 844 });

    await page.goto("/");

    await expect(page.getByRole("button", { name: "Show following" })).toHaveCount(0);
    await expect(page.locator(".viewer-shell")).not.toHaveClass(/viewer-shell--sidebar-open/);
    await expect(page.locator(".viewer-shell")).toHaveClass(/viewer-shell--following-disabled/);
    await expect(page.getByText("You're not following any channels yet.")).toHaveCount(0);
  });

  test("keeps populated following available behind the mobile drawer", async ({ page }) => {
    await mockViewerSession(page, true);
    await mockFollowing(page);
    await page.setViewportSize({ width: 390, height: 844 });

    await page.goto("/");

    const sidebarToggle = page.getByRole("button", { name: "Show following" });
    await expect(sidebarToggle).toBeVisible();
    await sidebarToggle.click();

    await expect(page.locator(".viewer-shell")).toHaveClass(/viewer-shell--sidebar-open/);
    await expect(page.locator(".viewer-sidebar").getByRole("link", { name: /ari wave/i })).toBeVisible();
  });
});
