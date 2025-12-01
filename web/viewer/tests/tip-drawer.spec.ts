import { expect, test } from "@playwright/test";

const playbackResponse = {
  channel: {
    id: "chan-42",
    ownerId: "owner-42",
    title: "Deep Space Beats",
    category: "Music",
    tags: ["lofi", "ambient"],
    liveState: "live",
    currentSessionId: "session-1",
    createdAt: new Date("2023-10-20T10:00:00Z").toISOString(),
    updatedAt: new Date("2023-10-21T11:00:00Z").toISOString()
  },
  owner: {
    id: "owner-42",
    displayName: "DJ Nova"
  },
  profile: {
    bio: "Streaming vinyl sets from a solar-powered cabin.",
    avatarUrl: undefined,
    bannerUrl: undefined
  },
  live: true,
  follow: {
    followers: 10,
    following: false
  },
  donationAddresses: [
    { currency: "eth", address: "0xabc123", note: "Main" },
    { currency: "btc", address: "bc1xyz" }
  ],
  subscription: {
    subscribers: 3,
    subscribed: false
  },
  playback: undefined,
  chat: {
    roomId: "room-1"
  }
};

const chatTranscript = {
  roomId: "room-1",
  participants: 2,
  messages: []
};

test.describe("Tip drawer", () => {
  test("keeps focus trapped while open", async ({ page }) => {
    await page.route("**/api/viewer/me", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          user: {
            id: "viewer-1",
            displayName: "Viewer",
            email: "viewer@example.com",
            roles: ["member"]
          },
          loginUrl: "https://auth.example.com/login",
          logoutUrl: "https://auth.example.com/logout"
        })
      });
    });

    await page.route("**/api/channels/chan-42/playback", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(playbackResponse)
      });
    });

    await page.route("**/api/channels/chan-42/vods", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ channelId: "chan-42", items: [] })
      });
    });

    await page.route("**/api/channels/chan-42/chat", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(chatTranscript)
      });
    });

    await page.goto("/channels/chan-42");

    await page.getByRole("button", { name: /send a tip/i }).click();
    const tipDialog = page.getByRole("dialog", { name: /send a tip/i });
    await expect(tipDialog).toBeVisible();

    await expect.poll(() => page.evaluate(() => document.activeElement?.id)).toBe("tip-amount");

    for (let i = 0; i < 10; i += 1) {
      await page.keyboard.press("Tab");
      await expect.poll(async () =>
        tipDialog.evaluate((node) => node.contains(document.activeElement))
      ).toBe(true);
    }

    await tipDialog.getByLabel("Amount").focus();

    for (let i = 0; i < 10; i += 1) {
      await page.keyboard.press("Shift+Tab");
      await expect.poll(async () =>
        tipDialog.evaluate((node) => node.contains(document.activeElement))
      ).toBe(true);
    }
  });
});
