import { expect, test, type Page } from "@playwright/test";

const MOBILE_WIDTHS = [320, 360, 390, 430] as const;
const generatedAt = new Date("2026-01-01T00:00:00Z").toISOString();

function directoryEntry(id: string, title: string, tagSuffix = id) {
  return {
    channel: {
      id,
      ownerId: `owner-${id}`,
      title,
      category: "Longform Mobile QA",
      tags: [`mobile-${tagSuffix}`, "very-long-tag-that-should-wrap-safely"],
      liveState: "live",
      currentSessionId: `session-${id}`,
      createdAt: generatedAt,
      updatedAt: generatedAt,
    },
    owner: {
      id: `owner-${id}`,
      displayName: `Creator ${tagSuffix.toUpperCase()} With A Long Name`,
    },
    profile: {
      bio: "Testing a compact mobile layout with enough descriptive copy to expose narrow-card overflow regressions.",
      avatarUrl: undefined,
      bannerUrl: undefined,
    },
    live: true,
    followerCount: 1234,
    viewerCount: 321,
  };
}

const mobileChannels = [
  directoryEntry("mobile-live", "A Very Long Live Channel Title That Still Needs To Fit"),
  directoryEntry("mobile-chat", "Tiny Viewport Chat And Playback Regression Room", "chat"),
  directoryEntry("mobile-creator", "Creator Setup Broadcast With Unusually Long Metadata", "creator"),
];

const mobileDirectoryResponse = {
  channels: mobileChannels,
  generatedAt,
};

const mobileCategoriesResponse = {
  categories: [
    {
      name: "Extremely Long Category Name",
      channelCount: 3,
      viewerCount: 963,
      tags: ["mobile", "layout"],
    },
    {
      name: "Chat",
      channelCount: 2,
      viewerCount: 444,
      tags: ["live"],
    },
  ],
  generatedAt,
};

function playbackResponse(channelId = "mobile-live") {
  const entry = mobileChannels.find((candidate) => candidate.channel.id === channelId) ?? mobileChannels[0];
  return {
    channel: {
      ...entry.channel,
      id: channelId,
      ownerId: "creator-owner",
      schedule: [
        {
          id: "schedule-mobile",
          title: "Small-screen regression stream with a long schedule title",
          startsAt: new Date("2026-01-02T17:30:00Z").toISOString(),
          durationMinutes: 90,
          description: "Schedule descriptions should wrap without widening the channel details panel.",
          createdAt: generatedAt,
          updatedAt: generatedAt,
        },
      ],
    },
    owner: { id: "creator-owner", displayName: "Mobile Creator With A Long Name" },
    profile: {
      bio: "A mobile-first channel page fixture with enough text to stress wrapping in hero, chat, and tab panels.",
      avatarUrl: undefined,
      bannerUrl: undefined,
    },
    live: true,
    follow: { followers: 4567, following: false },
    donationAddresses: [],
    subscription: { subscribers: 89, subscribed: false },
    playback: {
      sessionId: "session-mobile-live",
      startedAt: generatedAt,
      playbackUrl: "https://cdn.example.com/mobile/live/master.m3u8",
      protocol: "hls",
    },
    chat: { roomId: "room-mobile" },
    viewerCount: 321,
  };
}

async function mockViewerApis(page: Page, { signedIn = true }: { signedIn?: boolean } = {}) {
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
          id: "creator-owner",
          displayName: "Mobile Creator",
          email: "mobile@example.com",
          roles: ["creator"],
        },
        allowSelfSignup: true,
      }),
    });
  });

  await page.route("**/api/directory**", async (route) => {
    const url = new URL(route.request().url());
    if (url.pathname.endsWith("/categories")) {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(mobileCategoriesResponse),
      });
      return;
    }

    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(mobileDirectoryResponse),
    });
  });

  await page.route("**/api/channels", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify([
        {
          ...playbackResponse("mobile-live").channel,
          streamKey: "sk_mobile_layout_1234567890abcdef",
          ingestEndpoints: [
            "rtmp://ingest.example.com/live/mobile-layout-primary-with-a-long-hostname",
            "rtmp://backup.example.com/live/mobile-layout-secondary",
          ],
        },
      ]),
    });
  });

  await page.route("**/api/channels/*/playback", async (route) => {
    const segments = new URL(route.request().url()).pathname.split("/");
    const channelId = segments[segments.indexOf("channels") + 1] ?? "mobile-live";
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(playbackResponse(channelId)),
    });
  });

  await page.route("**/api/channels/*/sessions", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify([
        {
          id: "session-mobile-live",
          channelId: "mobile-live",
          startedAt: generatedAt,
          renditions: [],
          peakConcurrent: 42,
          playbackUrl: "https://cdn.example.com/mobile/live/master.m3u8",
        },
      ]),
    });
  });

  await page.route("**/api/channels/*/vods", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ channelId: "mobile-live", items: [] }),
    });
  });

  await page.route("**/api/channels/*/chat**", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify([
        {
          id: "chat-1",
          message: "ThisIsAReallyLongUnbrokenChatMessageThatShouldNotCreateHorizontalDocumentOverflowOnTinyScreens",
          sentAt: generatedAt,
          user: { id: "viewer-2", displayName: "Mobile Chat Tester" },
        },
      ]),
    });
  });
}

async function expectNoHorizontalOverflow(page: Page) {
  await expect
    .poll(async () =>
      page.evaluate(() => {
        const width = window.innerWidth;
        const documentWidth = document.documentElement.scrollWidth;
        const bodyWidth = document.body.scrollWidth;
        return Math.max(documentWidth, bodyWidth) - width;
      }),
    )
    .toBeLessThanOrEqual(1);
}

test.describe("mobile viewer layout", () => {
  test("keeps discovery chrome, following drawer, and auth overlay inside small viewports", async ({ page }) => {
    await mockViewerApis(page, { signedIn: false });

    for (const width of MOBILE_WIDTHS) {
      await page.setViewportSize({ width, height: 844 });
      await page.goto("/browse");

      await expect(page.getByRole("button", { name: "Open navigation menu" })).toBeVisible();
      await expectNoHorizontalOverflow(page);

      const followingToggle = page.getByRole("button", { name: "Show following" });
      await followingToggle.click();
      await expect(page.getByRole("button", { name: "Close following sidebar" })).toBeVisible();
      await expectNoHorizontalOverflow(page);
      await page.keyboard.press("Escape");

      await page.getByRole("button", { name: "Open navigation menu" }).click();
      await page.locator("#viewer-nav-menu").getByRole("button", { name: "Create account" }).click();
      await expect(page.getByRole("dialog", { name: "Create your BitRiver account" })).toBeVisible();
      await expectNoHorizontalOverflow(page);
      await page.getByRole("button", { name: "Close", exact: true }).click();
    }
  });

  test("keeps browse search, filters, and channel cards inside small viewports", async ({ page }) => {
    await mockViewerApis(page);

    for (const width of MOBILE_WIDTHS) {
      await page.setViewportSize({ width, height: 844 });
      await page.goto("/browse");

      await expect(page.getByRole("heading", { level: 1, name: /browse live channels/i })).toBeVisible();
      await expect(page.getByRole("button", { name: "Search" })).toBeVisible();
      await expect(page.getByRole("tab", { name: /trending/i })).toBeVisible();
      await expect(page.getByRole("heading", { level: 3, name: /a very long live channel/i }).first()).toBeVisible();
      await expectNoHorizontalOverflow(page);
    }
  });

  test("keeps channel watch, chat, and tabs usable inside small viewports", async ({ page }) => {
    await mockViewerApis(page);

    for (const width of MOBILE_WIDTHS) {
      await page.setViewportSize({ width, height: 844 });
      await page.goto("/channels/mobile-live");

      await expect(page.getByRole("navigation", { name: "Watch page sections" })).toBeVisible();
      await expect(page.getByRole("heading", { name: "Live chat" })).toBeVisible();
      await expect(page.getByRole("tab", { name: "About" })).toBeVisible();
      await expect(page.getByRole("button", { name: "Show following" })).toHaveCount(0);
      await expectNoHorizontalOverflow(page);
    }
  });

  test("keeps creator live setup forms and long copy targets inside small viewports", async ({ page }) => {
    await mockViewerApis(page);

    for (const width of MOBILE_WIDTHS) {
      await page.setViewportSize({ width, height: 844 });
      await page.goto("/creator/live/mobile-live");

      await expect(page.getByRole("heading", { level: 3, name: "1) Channel" })).toBeVisible();
      await expect(page.getByLabel("Preferred ingest URL")).toBeVisible();
      await expect(page.getByLabel("Stream key")).toBeVisible();
      await expect(page.getByLabel("Viewer link")).toBeVisible();
      await expect(page.getByRole("button", { name: "Copy OBS settings" })).toBeVisible();
      await expectNoHorizontalOverflow(page);
    }
  });
});
