import AxeBuilder from "@axe-core/playwright";
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

const emptyResponse = {
  channels: [],
  generatedAt: new Date("2023-10-21T12:45:00Z").toISOString()
};

test.beforeEach(async ({ page }) => {
  await page.route("**/api/viewer/me", async (route) => {
    await route.fulfill({ status: 401, body: "Unauthorized" });
  });

  await page.route("**/api/directory**", async (route) => {
    const url = new URL(route.request().url());
    const query = url.searchParams.get("q");
    const category = url.searchParams.get("category");
    const body = query === "nothing" ? emptyResponse : query || category === "Gaming" ? searchResponse : directoryResponse;
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(body) });
  });
});

test("directory page renders accessible markup and supports search", async ({ page }) => {
  await page.goto("/browse");

  await expect(page.getByRole("heading", { level: 1, name: /browse live channels/i })).toBeVisible();
  await expect(page.getByRole("heading", { level: 3, name: "Deep Space Beats" }).first()).toBeVisible();

  await page.getByLabel("Search channels").fill("retro");
  await page.getByRole("main").getByRole("button", { name: "Search" }).click();

  await expect(page.getByRole("heading", { level: 3, name: "Retro Speedruns" }).first()).toBeVisible();
  await expect(page.getByRole("heading", { level: 3, name: "Deep Space Beats" })).toHaveCount(0);

  const results = await new AxeBuilder({ page })
    .include("main")
    .withTags(["wcag2a", "wcag2aa"])
    .analyze();
  expect(results.violations).toEqual([]);
});

test("directory page keeps topic URLs, reset, and empty states clear", async ({ page }) => {
  await page.goto("/browse?topic=Gaming");

  await expect(page.getByRole("heading", { level: 2, name: "Gaming channels" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Gaming" })).toHaveAttribute("aria-pressed", "true");
  await expect(page).toHaveURL(/\/browse\?topic=Gaming$/);

  await page.getByRole("button", { name: "Reset" }).click();
  await expect(page).toHaveURL(/\/browse$/);
  await expect(page.getByRole("heading", { level: 2, name: "All channels" })).toBeVisible();

  await page.getByLabel("Search channels").fill("nothing");
  await page.getByRole("main").getByRole("button", { name: "Search" }).click();

  await expect(page.getByRole("heading", { level: 2, name: "No matches" })).toBeVisible();
  await expect(page.getByText("Clear filters or try another search.")).toBeVisible();
});
