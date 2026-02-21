import { expect, test } from "@playwright/test";

test.describe("creator dashboard", () => {
  test("registers uploads through the creator dashboard", async ({ page }) => {
    const channelId = "chan-creator";

    await page.route("**/api/viewer/me", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          user: { id: "creator-1", displayName: "Creator", email: "creator@example.com", roles: ["creator"] },
          loginUrl: "https://auth.example.com/login",
          logoutUrl: "https://auth.example.com/logout",
        }),
      });
    });

    await page.route(`**/api/channels/${channelId}/playback`, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          channel: {
            id: channelId,
            ownerId: "creator-1",
            title: "Creator Control Centre",
            category: "Talk Shows",
            tags: ["behind-the-scenes"],
            liveState: "live",
            currentSessionId: "session-creator-1",
            createdAt: new Date("2024-04-20T10:00:00Z").toISOString(),
            updatedAt: new Date("2024-04-20T12:00:00Z").toISOString(),
          },
          owner: { id: "creator-1", displayName: "Creator" },
          profile: { bio: "Sharing production tips", avatarUrl: undefined, bannerUrl: undefined },
          live: true,
          follow: { followers: 10, following: true },
          donationAddresses: [],
          subscription: { subscribers: 0, subscribed: true },
          playback: undefined,
          chat: { roomId: "room-creator" },
        }),
      });
    });

    type UploadItem = {
      id: string;
      channelId: string;
      title?: string;
      filename: string;
      sizeBytes: number;
      status: string;
      progress: number;
      createdAt: string;
      updatedAt: string;
      error?: string;
    };

    let uploadItems: UploadItem[] = [];
    let uploadAttempts = 0;

    await page.route("**/api/uploads**", async (route) => {
      const { method, url } = route.request();

      if (method === "GET") {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify(uploadItems),
        });
        return;
      }

      if (method === "POST") {
        uploadAttempts += 1;
        const payload = route.request().postDataJSON();

        if (uploadAttempts === 1) {
          await route.fulfill({
            status: 500,
            contentType: "application/json",
            body: JSON.stringify({ message: "Unable to create upload" }),
          });
          return;
        }

        const newItem: UploadItem = {
          id: `upload-${uploadAttempts}`,
          channelId,
          title: payload?.title ?? payload?.filename ?? "Untitled",
          filename: payload?.filename ?? "uploaded.bin",
          sizeBytes: payload?.sizeBytes ?? 0,
          status: "processing",
          progress: 12,
          createdAt: new Date("2024-04-20T12:00:00Z").toISOString(),
          updatedAt: new Date("2024-04-20T12:00:00Z").toISOString(),
        };
        uploadItems = [newItem];
        await route.fulfill({
          status: 201,
          contentType: "application/json",
          body: JSON.stringify(newItem),
        });
        return;
      }

      if (method === "DELETE") {
        uploadItems = [];
        await route.fulfill({ status: 204, contentType: "application/json", body: "" });
        return;
      }

      await route.fulfill({ status: 404 });
    });

    await page.goto(`/creator/uploads/${channelId}`);

    await expect(page.getByRole("heading", { level: 2, name: /manage uploads/i })).toBeVisible();
    await expect(page.getByText(/upload manager/i)).toBeVisible();

    const uploadManager = page.getByRole("heading", { level: 3, name: /upload manager/i }).locator("xpath=ancestor::section[1]");
    const submitButton = uploadManager.getByRole("button", { name: /register upload|submitting…/i });

    await page.getByLabel("Title").fill("Post-show recap");
    await page.getByLabel("Filename").fill("recap.mp4");
    await page.getByLabel("Playback URL (optional)").fill("https://cdn.example.com/recap.m3u8");
    await page.getByLabel("Size (bytes)").fill("1048576");

    await page.getByRole("button", { name: /add metadata/i }).click();
    const keyInputs = page.getByPlaceholder("Key");
    await keyInputs.nth(1).fill("season");
    const valueInputs = page.getByPlaceholder("Value");
    await valueInputs.nth(1).fill("2");

    const firstSubmitResponse = page.waitForResponse(
      (response) => response.request().method() === "POST" && response.url().includes("/api/uploads") && response.status() === 500,
    );
    await Promise.all([
      firstSubmitResponse,
      submitButton.click(),
    ]);

    await expect(submitButton).toHaveText("Submitting…");
    await expect(submitButton).toHaveText("Register upload");
    await expect(uploadManager.getByText(/unable to create upload/i)).toBeVisible();

    await submitButton.click();

    await expect.poll(() => uploadItems.length).toBeGreaterThan(0);
    await expect(page.getByText(/processing · 12%/i)).toBeVisible();
    await expect(page.getByRole("button", { name: /refresh/i })).toBeEnabled();
  });

  test("updates a stream schedule from the live dashboard", async ({ page }) => {
    const channelId = "chan-live";

    await page.route("**/api/viewer/me", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          user: { id: "creator-live", displayName: "Live Host", email: "live@example.com", roles: ["creator"] },
          loginUrl: "https://auth.example.com/login",
          logoutUrl: "https://auth.example.com/logout",
        }),
      });
    });

    const playback = {
      channel: {
        id: channelId,
        ownerId: "creator-live",
        title: "Mission Briefing",
        category: "Science & Tech",
        tags: ["space", "updates"],
        liveState: "offline",
        createdAt: new Date("2024-04-18T10:00:00Z").toISOString(),
        updatedAt: new Date("2024-04-19T11:00:00Z").toISOString(),
      },
      owner: { id: "creator-live", displayName: "Live Host" },
      profile: { bio: "Live mission updates", avatarUrl: undefined, bannerUrl: undefined },
      live: false,
      follow: { followers: 50, following: true },
      donationAddresses: [],
      subscription: { subscribers: 5, subscribed: true },
      playback: {
        sessionId: "session-live-1",
        startedAt: new Date("2024-04-18T10:00:00Z").toISOString(),
        playbackUrl: "https://cdn.example.com/live/briefing.m3u8",
        originUrl: "https://cdn.example.com/thumbs/briefing.jpg",
        protocol: "hls",
        latencyMode: "low-latency",
      },
      chat: { roomId: "room-live" },
    };

    await page.route(`**/api/channels/${channelId}/playback`, async (route) => {
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(playback) });
    });

    let sessionCalls = 0;
    const sessionResponse = [
      {
        id: "session-live-1",
        channelId,
        startedAt: new Date("2024-04-19T13:00:00Z").toISOString(),
        renditions: ["1080p", "720p"],
        peakConcurrent: 1200,
      },
      {
        id: "session-live-0",
        channelId,
        startedAt: new Date("2024-04-18T10:00:00Z").toISOString(),
        endedAt: new Date("2024-04-18T12:00:00Z").toISOString(),
        renditions: ["1080p"],
        peakConcurrent: 900,
      },
    ];

    await page.route(`**/api/channels/${channelId}/sessions`, async (route) => {
      sessionCalls += 1;
      if (sessionCalls === 1) {
        await route.fulfill({
          status: 500,
          contentType: "application/json",
          body: JSON.stringify({ message: "Unable to load ingest details" }),
        });
        return;
      }
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(sessionResponse) });
    });

    await page.route("**/api/channels", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify([
          {
            id: channelId,
            ownerId: "creator-live",
            title: playback.channel.title,
            category: playback.channel.category,
            tags: playback.channel.tags,
            liveState: playback.channel.liveState,
            createdAt: playback.channel.createdAt,
            updatedAt: playback.channel.updatedAt,
            streamKey: "sk_live_123",
            ingestEndpoints: ["rtmp://ingest.example.com/live", "rtmp://backup.example.com/live"],
          },
          {
            id: "chan-side", ownerId: "creator-live", title: "Backup channel", category: "Talk", tags: [], liveState: "offline",
            createdAt: playback.channel.createdAt, updatedAt: playback.channel.updatedAt, streamKey: "sk_side_123",
            ingestEndpoints: ["rtmp://side.example.com/live"],
          },
        ]),
      });
    });

    let lastUpdatePayload: any;
    let updateAttempts = 0;
    await page.route(`**/api/channels/${channelId}`, async (route) => {
      updateAttempts += 1;
      if (route.request().method() === "PATCH") {
        if (updateAttempts === 1) {
          await route.fulfill({
            status: 500,
            contentType: "application/json",
            body: JSON.stringify({ message: "Unable to update stream title" }),
          });
          return;
        }
        lastUpdatePayload = route.request().postDataJSON();
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ ...playback.channel, ...lastUpdatePayload, streamKey: "sk_live_123" }),
        });
        return;
      }

      await route.fulfill({ status: 404 });
    });

    await page.goto(`/creator/live/${channelId}`);

    await expect(page.getByRole("heading", { level: 2, name: /go live with/i })).toBeVisible();
    await expect(page.getByText(/unable to load ingest details/i)).toBeVisible();

    await page.getByRole("button", { name: /refresh details/i }).click();

    await expect(page.getByText(/unable to load ingest details/i)).not.toBeVisible();
    await expect(page.getByText(/idle/i)).toBeVisible();
    await expect(page.getByText(/last transition unknown/i)).toBeVisible();
    await expect(page.getByText(/session started/i)).toBeVisible();

    const titleInput = page.getByLabel("Stream title");
    await titleInput.fill("Scheduled Mission Update");
    await page.getByRole("button", { name: /save title/i }).click();
    await expect(page.getByText(/unable to update stream title/i)).toBeVisible();

    await page.getByRole("button", { name: /save title/i }).click();
    await expect(page.getByText(/stream title updated/i)).toBeVisible();
    await expect.poll(() => lastUpdatePayload?.title).toBe("Scheduled Mission Update");

    await expect(page.getByText(/primary ingest/i)).toBeVisible();
    await expect(page.getByText(/backup ingest/i)).toBeVisible();
  });

  test("shows error lifecycle state when session signals are out of sync", async ({ page }) => {
    const channelId = "chan-lifecycle-error";

    await page.route("**/api/viewer/me", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          user: { id: "creator-live", displayName: "Live Host", email: "live@example.com", roles: ["creator"] },
          loginUrl: "https://auth.example.com/login",
          logoutUrl: "https://auth.example.com/logout",
        }),
      });
    });

    await page.route(`**/api/channels/${channelId}/playback`, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          channel: {
            id: channelId,
            ownerId: "creator-live",
            title: "Lifecycle Watch",
            category: "Talk",
            tags: [],
            liveState: "offline",
            currentSessionId: "session-expected",
            createdAt: new Date("2024-04-20T10:00:00Z").toISOString(),
            updatedAt: new Date("2024-04-20T12:00:00Z").toISOString(),
          },
          owner: { id: "creator-live", displayName: "Live Host" },
          profile: { bio: "Live mission updates", avatarUrl: undefined, bannerUrl: undefined },
          live: false,
          follow: { followers: 50, following: true },
          donationAddresses: [],
          subscription: { subscribers: 5, subscribed: true },
          playback: undefined,
          chat: { roomId: "room-live" },
        }),
      });
    });

    await page.route(`**/api/channels/${channelId}/sessions`, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify([
          {
            id: "session-other",
            channelId,
            startedAt: new Date("2024-04-19T13:00:00Z").toISOString(),
            renditions: ["1080p"],
            peakConcurrent: 1200,
          },
        ]),
      });
    });

    await page.route("**/api/channels", async (route) => {
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify([]) });
    });

    await page.goto(`/creator/live/${channelId}`);

    await expect(page.getByText("Error")).toBeVisible();
    await expect(page.getByText(/ingest lost: channel session signal is out of sync/i)).toBeVisible();
  });
});
