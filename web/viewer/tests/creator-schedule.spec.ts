import { expect, test } from "@playwright/test";

test.describe("creator schedule management", () => {
  test("refreshes ingest details and updates a scheduled stream", async ({ page }) => {
    const channelId = "creator-schedule";

    await page.route("**/api/viewer/me", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          user: { id: "creator-schedule-owner", displayName: "Scheduler", roles: ["creator"] },
          loginUrl: "https://auth.example.com/login",
          logoutUrl: "https://auth.example.com/logout",
        }),
      });
    });

    const playback = {
      channel: {
        id: channelId,
        ownerId: "creator-schedule-owner",
        title: "Deep Dive", // initial title should be editable
        category: "Science & Tech",
        tags: ["space"],
        liveState: "offline",
        createdAt: new Date("2024-05-10T10:00:00Z").toISOString(),
        updatedAt: new Date("2024-05-10T12:00:00Z").toISOString(),
      },
      owner: { id: "creator-schedule-owner", displayName: "Scheduler" },
      profile: { bio: "Planning streams", avatarUrl: undefined, bannerUrl: undefined },
      live: false,
      follow: { followers: 10, following: true },
      donationAddresses: [],
      subscription: { subscribers: 2, subscribed: true },
      playback: {
        sessionId: "session-schedule-1",
        startedAt: new Date("2024-05-10T10:00:00Z").toISOString(),
        playbackUrl: "https://cdn.example.com/live/deep-dive.m3u8",
        originUrl: "https://cdn.example.com/thumbs/deep-dive.jpg",
        protocol: "hls",
        latencyMode: "low-latency",
      },
      chat: { roomId: "schedule-room" },
    };

    await page.route(`**/api/channels/${channelId}/playback`, async (route) => {
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(playback) });
    });

    let sessionCalls = 0;
    const sessions = [
      {
        id: "session-schedule-1",
        channelId,
        startedAt: new Date("2024-05-10T14:00:00Z").toISOString(),
        renditions: ["1080p", "720p"],
        peakConcurrent: 450,
      },
      {
        id: "session-schedule-0",
        channelId,
        startedAt: new Date("2024-05-09T10:00:00Z").toISOString(),
        endedAt: new Date("2024-05-09T12:00:00Z").toISOString(),
        renditions: ["1080p"],
        peakConcurrent: 300,
      },
    ];

    await page.route(`**/api/channels/${channelId}/sessions`, async (route) => {
      sessionCalls += 1;
      if (sessionCalls === 1) {
        await route.fulfill({
          status: 500,
          contentType: "application/json",
          body: JSON.stringify({ message: "Sessions unavailable" }),
        });
        return;
      }
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(sessions) });
    });

    let managedCalls = 0;
    const managedChannels = [
      {
        id: channelId,
        ownerId: "creator-schedule-owner",
        title: playback.channel.title,
        category: playback.channel.category,
        tags: playback.channel.tags,
        liveState: playback.channel.liveState,
        createdAt: playback.channel.createdAt,
        updatedAt: playback.channel.updatedAt,
        streamKey: "sk_schedule_primary",
        ingestEndpoints: ["rtmp://ingest.example.com/live", "rtmp://backup.example.com/live"],
      },
    ];

    await page.route("**/api/channels", async (route) => {
      managedCalls += 1;
      if (managedCalls === 1) {
        await route.fulfill({
          status: 500,
          contentType: "application/json",
          body: JSON.stringify({ message: "Channel list offline" }),
        });
        return;
      }
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(managedChannels) });
    });

    let updateAttempts = 0;
    let lastPatch: any;
    await page.route(`**/api/channels/${channelId}`, async (route) => {
      if (route.request().method() === "PATCH") {
        updateAttempts += 1;
        if (updateAttempts === 1) {
          await route.fulfill({
            status: 500,
            contentType: "application/json",
            body: JSON.stringify({ message: "Schedule locked" }),
          });
          return;
        }
        lastPatch = route.request().postDataJSON();
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ ...managedChannels[0], ...lastPatch }),
        });
        return;
      }

      await route.fulfill({ status: 404 });
    });

    await page.goto(`/creator/live/${channelId}`);

    await expect(page.getByRole("heading", { level: 2, name: /go live with deep dive/i })).toBeVisible();
    await expect(page.getByText(/sessions unavailable/i)).toBeVisible();
    await expect(page.getByText(/channel list offline/i)).toBeVisible();

    await page.getByRole("button", { name: /refresh details/i }).click();

    await expect(page.getByText(/sessions unavailable/i)).toBeHidden();
    await expect(page.getByText(/channel list offline/i)).toBeHidden();
    await expect(page.getByText(/idle/i)).toBeVisible();
    await expect(page.getByText(/last transition unknown/i)).toBeVisible();
    await expect(page.getByText(/session started/i)).toBeVisible();

    const titleInput = page.getByLabel("Stream title");
    await titleInput.fill("Scheduled Deep Dive");
    await page.getByRole("button", { name: /save title/i }).click();
    await expect(page.getByText(/schedule locked/i)).toBeVisible();

    await page.getByRole("button", { name: /save title/i }).click();
    await expect(page.getByText(/stream title updated/i)).toBeVisible();
    await expect.poll(() => lastPatch?.title).toBe("Scheduled Deep Dive");

    const streamKeySection = page.getByText("Stream key").locator("..");
    await streamKeySection.getByRole("button", { name: "Show", exact: true }).click();
    await expect(page.getByText(/sk_schedule_primary/)).toBeVisible();
    await expect(page.getByText(/primary ingest/i)).toBeVisible();
    await expect(page.getByText(/backup ingest/i)).toBeVisible();
  });
});
