import { expect, test } from "@playwright/test";

const channelId = "chan-live";

const playbackResponse = {
  channel: {
    id: channelId,
    ownerId: "owner-100",
    title: "Mission Control Live",
    category: "Science & Tech",
    tags: ["space", "launch"],
    liveState: "live",
    currentSessionId: "session-rocket",
    createdAt: new Date("2024-03-12T12:00:00Z").toISOString(),
    updatedAt: new Date("2024-03-12T12:30:00Z").toISOString()
  },
  owner: {
    id: "owner-100",
    displayName: "Launch Director"
  },
  profile: {
    bio: "Live coverage from the launch pad.",
    avatarUrl: undefined,
    bannerUrl: undefined,
    socialLinks: []
  },
  donationAddresses: [],
  live: true,
  follow: {
    followers: 120,
    following: false
  },
  subscription: {
    subscribers: 15,
    subscribed: false
  },
  playback: {
    sessionId: "session-rocket",
    startedAt: new Date("2024-03-12T12:00:00Z").toISOString(),
    playbackUrl: "https://cdn.example.com/live/mission-control.m3u8",
    originUrl: "https://cdn.example.com/thumbnails/mission-control.jpg",
    protocol: "hls",
    latencyMode: "low-latency",
    renditions: [
      {
        name: "source",
        manifestUrl: "https://cdn.example.com/live/mission-control-source.m3u8",
        bitrate: 6400
      }
    ]
  },
  viewerCount: 128,
  chat: {
    roomId: "room-42"
  }
};

const initialChatMessages = [
  {
    id: "msg-1",
    channelId,
    userId: "host-1",
    content: "Welcome aboard, mission watchers!",
    createdAt: new Date("2024-03-12T12:01:00Z").toISOString()
  },
  {
    id: "msg-2",
    channelId,
    userId: "viewer-2",
    content: "Engine chilldown has started.",
    createdAt: new Date("2024-03-12T12:02:30Z").toISOString()
  }
];

test.describe("live stream playback", () => {
  let postedMessages: string[] = [];

  test.beforeEach(async ({ page }) => {
    postedMessages = [];

    await page.route("**/api/viewer/me", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          user: {
            id: "viewer-1",
            displayName: "Viewer One",
            email: "viewer@example.com",
            roles: ["member"]
          },
          loginUrl: "https://auth.example.com/login",
          logoutUrl: "https://auth.example.com/logout"
        })
      });
    });

    await page.route(`**/api/channels/${channelId}/playback`, async (route) => {
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(playbackResponse) });
    });

    await page.route(`**/api/channels/${channelId}/vods`, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ channelId, items: [] })
      });
    });

    await page.route(`**/api/channels/${channelId}/chat**`, async (route) => {
      if (route.request().method() === "GET") {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify(initialChatMessages)
        });
        return;
      }

      const body = route.request().postDataJSON() as { userId: string; content: string };
      postedMessages.push(body.content);
      const nextId = `msg-${initialChatMessages.length + postedMessages.length}`;
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          id: nextId,
          channelId,
          userId: body.userId,
          content: body.content,
          createdAt: new Date("2024-03-12T12:05:00Z").toISOString()
        })
      });
    });
  });

  test("plays the live feed and relays chat", async ({ page }) => {
    await page.goto(`/channels/${channelId}`);

    await expect(page.getByRole("heading", { level: 1, name: playbackResponse.channel.title })).toBeVisible();
    await expect(page.getByText("128 viewers")).toBeVisible();

    const video = page.locator("video");
    await expect(video).toBeVisible();
    await expect(video).toHaveAttribute("controls", "");
    await expect(video).toHaveAttribute("src", playbackResponse.playback?.playbackUrl ?? "");
    await expect(video).toHaveAttribute("poster", playbackResponse.playback?.originUrl ?? "");

    const chatLog = page.getByRole("log");
    await expect(chatLog).toContainText("Welcome aboard, mission watchers!");
    await expect(chatLog).toContainText("Engine chilldown has started.");
    await expect(page.getByText("2 messages")).toBeVisible();

    const chatInput = page.getByLabel("Chat message");
    await chatInput.fill("Systems green across the board.");
    await page.getByRole("button", { name: "Send" }).click();

    await expect.poll(() => postedMessages[0]).toBe("Systems green across the board.");
    await expect(chatLog).toContainText("Systems green across the board.");
    await expect(page.getByText("3 messages")).toBeVisible();
  });
});
