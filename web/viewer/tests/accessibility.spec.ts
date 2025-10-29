import { injectAxe, checkA11y } from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

const directoryResponse = {
  channels: [
    {
      channel: {
        id: "chan-1",
        ownerId: "owner-1",
        title: "Deep Space Beats",
        category: "Music",
        tags: ["lofi", "ambient"],
        liveState: "live",
        currentSessionId: "session-1",
        createdAt: new Date("2023-10-20T10:00:00Z").toISOString(),
        updatedAt: new Date("2023-10-21T11:00:00Z").toISOString()
      },
      owner: {
        id: "owner-1",
        displayName: "DJ Nova"
      },
      profile: {
        bio: "Streaming vinyl sets from a solar-powered cabin.",
        avatarUrl: undefined,
        bannerUrl: undefined
      },
      live: true,
      followerCount: 12
    }
  ],
  generatedAt: new Date("2023-10-21T11:00:00Z").toISOString()
};

const searchResponse = {
  channels: [
    {
      channel: {
        id: "chan-2",
        ownerId: "owner-2",
        title: "Retro Speedruns",
        category: "Gaming",
        tags: ["speedrun", "retro"],
        liveState: "live",
        currentSessionId: "session-2",
        createdAt: new Date("2023-10-18T18:00:00Z").toISOString(),
        updatedAt: new Date("2023-10-21T12:30:00Z").toISOString()
      },
      owner: {
        id: "owner-2",
        displayName: "PixelPro"
      },
      profile: {
        bio: "Tool-assisted runs from the golden age of consoles.",
        avatarUrl: undefined,
        bannerUrl: undefined
      },
      live: true,
      followerCount: 8
    }
  ],
  generatedAt: new Date("2023-10-21T12:30:00Z").toISOString()
};

test.beforeEach(async ({ page }) => {
  await page.route("**/api/auth/session", async (route) => {
    await route.fulfill({ status: 401, body: "Unauthorized" });
  });

  await page.route("**/api/directory**", async (route) => {
    const url = new URL(route.request().url());
    const body = url.searchParams.has("q") ? searchResponse : directoryResponse;
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(body) });
  });
});

test("directory page renders accessible markup and supports search", async ({ page }) => {
  await page.goto("/");

  await expect(page.getByRole("heading", { level: 1, name: /discover live channels/i })).toBeVisible();
  await expect(page.getByRole("heading", { level: 3, name: "Deep Space Beats" })).toBeVisible();

  await page.fill("input[type=search]", "retro");
  await page.click("button:has-text('Search')");

  await expect(page.getByRole("heading", { level: 3, name: "Retro Speedruns" })).toBeVisible();
  await expect(page.getByRole("heading", { level: 3, name: "Deep Space Beats" })).toHaveCount(0);

  await injectAxe(page);
  await checkA11y(page, "main", {
    runOnly: {
      type: "tag",
      values: ["wcag2a", "wcag2aa"]
    }
  });
});
