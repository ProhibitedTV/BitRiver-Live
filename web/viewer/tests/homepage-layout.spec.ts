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
  test("keeps the following rail aligned with the featured stage", async ({ page }) => {
    await mockViewerSession(page, false);
    await page.setViewportSize({ width: 1600, height: 1100 });
    await page.goto("/");

    const sidebar = page.locator(".viewer-sidebar");
    const hero = page.locator(".home-hero");

    await expect(sidebar).toBeVisible();
    await expect(hero).toBeVisible();
    await expect(page.locator(".viewer-shell")).toHaveClass(/viewer-shell--desktop/);
    await expect(page.locator(".viewer-shell")).toHaveClass(/viewer-shell--following-persistent/);
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
      };
    });

    expect(layout.sidebarTop).not.toBeNull();
    expect(layout.heroTop).not.toBeNull();
    expect(Math.abs((layout.sidebarTop ?? 0) - (layout.heroTop ?? 0))).toBeLessThanOrEqual(8);
  });

  test("renders followed channels in the desktop discovery sidebar", async ({ page }) => {
    await mockViewerSession(page, true);
    await mockFollowing(page);
    await page.setViewportSize({ width: 1440, height: 1000 });

    await page.goto("/");

    await expect(page.locator(".viewer-shell")).toHaveClass(/viewer-shell--following-persistent/);
    await expect(page.locator(".viewer-sidebar").getByRole("link", { name: /ari wave/i })).toBeVisible();
    await expect(page.getByText("1 followed")).toBeVisible();
  });

  test("keeps empty following lightweight behind the mobile drawer", async ({ page }) => {
    await mockViewerSession(page, true);
    await mockFollowing(page, []);
    await page.setViewportSize({ width: 390, height: 844 });

    await page.goto("/");

    const sidebarToggle = page.getByRole("button", { name: "Show following" });
    await expect(sidebarToggle).toBeVisible();
    await expect(page.locator(".viewer-shell")).not.toHaveClass(/viewer-shell--sidebar-open/);

    await sidebarToggle.click();

    await expect(page.locator(".viewer-shell")).toHaveClass(/viewer-shell--sidebar-open/);
    await expect(page.getByText("You're not following any channels yet.")).toBeVisible();
  });
});
