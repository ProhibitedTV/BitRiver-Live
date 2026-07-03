import { expect, test } from "@playwright/test";

test.describe("creator live setup", () => {
  test("guides a creator from stream settings to a live preview and share link", async ({ page }) => {
    const channelId = "creator-live-setup";
    const backupChannelId = "creator-live-backup";
    const streamKey = "sk_live_setup_123";
    const ingestUrl = "rtmp://ingest.example.com/live";
    const livePlaybackUrl = "https://cdn.example.com/live/master.m3u8";
    let playbackChecks = 0;
    let sessionChecks = 0;

    await page.addInitScript(() => {
      const clipboardWrites: string[] = [];
      Object.defineProperty(window, "__clipboardWrites", {
        value: clipboardWrites,
        writable: false
      });
      Object.defineProperty(navigator, "clipboard", {
        value: {
          writeText: async (text: string) => {
            clipboardWrites.push(text);
          }
        },
        configurable: true
      });
    });

    await page.route("**/api/viewer/me", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          user: {
            id: "creator-live-owner",
            displayName: "Live Owner",
            email: "owner@example.com",
            roles: ["creator"]
          },
          loginUrl: "https://auth.example.com/login",
          logoutUrl: "https://auth.example.com/logout"
        })
      });
    });

    await page.route(`**/api/channels/${channelId}/playback`, async (route) => {
      playbackChecks += 1;
      const isLive = playbackChecks > 2;
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          channel: {
            id: channelId,
            ownerId: "creator-live-owner",
            title: "Creator Live Setup",
            category: "Science & Tech",
            tags: ["setup"],
            liveState: isLive ? "live" : "offline",
            currentSessionId: isLive ? "session-live-1" : undefined,
            createdAt: new Date("2024-05-01T10:00:00Z").toISOString(),
            updatedAt: new Date("2024-05-01T10:30:00Z").toISOString()
          },
          owner: { id: "creator-live-owner", displayName: "Live Owner" },
          profile: { bio: "Creator setup", avatarUrl: undefined, bannerUrl: undefined },
          live: isLive,
          follow: { followers: 0, following: false },
          donationAddresses: [],
          subscription: { subscribers: 0, subscribed: false },
          playback: isLive
            ? {
                sessionId: "session-live-1",
                startedAt: new Date("2024-05-01T10:35:00Z").toISOString(),
                playbackUrl: livePlaybackUrl,
                protocol: "hls"
              }
            : undefined,
          chat: { roomId: "room-live-setup" }
        })
      });
    });

    await page.route(`**/api/channels/${channelId}/sessions`, async (route) => {
      sessionChecks += 1;
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(
          sessionChecks > 1
            ? [
                {
                  id: "session-live-1",
                  channelId,
                  startedAt: new Date("2024-05-01T10:35:00Z").toISOString(),
                  renditions: [],
                  peakConcurrent: 0,
                  playbackUrl: livePlaybackUrl
                }
              ]
            : []
        )
      });
    });

    await page.route(`**/api/channels/${channelId}/sessions/status`, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ status: "idle" })
      });
    });

    await page.route("**/api/channels", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify([
          {
            id: channelId,
            ownerId: "creator-live-owner",
            title: "Creator Live Setup",
            category: "Science & Tech",
            tags: ["setup"],
            liveState: "offline",
            createdAt: new Date("2024-05-01T10:00:00Z").toISOString(),
            updatedAt: new Date("2024-05-01T10:30:00Z").toISOString(),
            streamKey,
            ingestEndpoints: [ingestUrl, "rtmp://backup.example.com/live"]
          },
          {
            id: backupChannelId,
            ownerId: "creator-live-owner",
            title: "Creator Live Backup",
            category: "Science & Tech",
            tags: ["backup"],
            liveState: "offline",
            createdAt: new Date("2024-05-01T10:00:00Z").toISOString(),
            updatedAt: new Date("2024-05-01T10:30:00Z").toISOString(),
            streamKey: "sk_live_backup_456",
            ingestEndpoints: ["rtmp://backup-channel.example.com/live"]
          }
        ])
      });
    });

    await page.goto(`/creator/live/${channelId}`);

    await expect(page.getByRole("heading", { level: 2, name: /creator live setup studio/i })).toBeVisible();
    const studioNav = page.getByRole("navigation", { name: /creator live setup channel tools/i });
    await expect(studioNav.getByRole("link", { name: "Public preview" })).toHaveAttribute("href", `/channels/${channelId}`);
    await expect(studioNav.getByRole("link", { name: "Uploads" })).toHaveAttribute("href", `/creator/uploads/${channelId}`);
    await expect(studioNav.getByRole("link", { name: "Share link" })).toHaveAttribute("href", `/creator/live/${channelId}#channel-share`);

    await expect(page.getByRole("heading", { level: 3, name: "1) Channel" })).toBeVisible();
    await expect(page.getByRole("heading", { level: 3, name: "2) Stream settings" })).toBeVisible();
    await expect(page.getByRole("heading", { level: 3, name: "3) Go live" })).toBeVisible();
    await expect(page.getByRole("heading", { level: 3, name: "4) Share" })).toBeVisible();

    await expect(page.getByLabel("Current channel")).toHaveValue("Creator Live Setup");
    await expect(page.getByLabel("Switch channel")).toHaveValue(channelId);
    await expect(page.getByLabel("Preferred ingest URL")).toHaveValue(ingestUrl);

    const streamKeyInput = page.getByLabel("Stream key");
    const revealButton = page.getByRole("button", { name: "Reveal", exact: true });
    const copyKeyButton = page.getByRole("button", { name: "Copy key", exact: true });
    const copyIngestButton = page.getByTestId("copy-preferred-ingest-endpoint");
    const copyObsButton = page.getByTestId("copy-obs-settings");
    const copyViewerLinkButton = page.getByTestId("copy-viewer-link");

    await expect(streamKeyInput).toHaveValue("********");
    await expect(copyObsButton).toBeVisible();
    await expect(
      page.getByTestId("test-stream-status-card").getByText("Waiting for stream", { exact: true })
    ).toBeVisible();
    await expect(
      page.getByText(
        "Start OBS. Preview appears here."
      )
    ).toBeVisible();
    await expect(page.getByText("Service: Custom")).toBeVisible();
    await expect(page.getByText(`Server: ${ingestUrl}`)).toBeVisible();
    await expect(page.getByText("Stream key: reveal or copy above.")).toBeVisible();

    await expect(copyKeyButton).toBeEnabled();
    await copyKeyButton.click();
    await expect(page.getByText("Copied", { exact: true })).toBeVisible();

    let clipboardWrites = await page.evaluate(
      () => (window as typeof window & { __clipboardWrites: string[] }).__clipboardWrites
    );
    expect(clipboardWrites).toContain(streamKey);

    await copyIngestButton.click();
    await expect(copyIngestButton).toHaveText("Copied");

    clipboardWrites = await page.evaluate(
      () => (window as typeof window & { __clipboardWrites: string[] }).__clipboardWrites
    );
    expect(clipboardWrites).toContain(ingestUrl);

    await copyObsButton.click();
    await expect(page.getByText("Copied OBS settings", { exact: true })).toBeVisible();

    clipboardWrites = await page.evaluate(
      () => (window as typeof window & { __clipboardWrites: string[] }).__clipboardWrites
    );
    expect(clipboardWrites).toContain(`Service: Custom\nServer: ${ingestUrl}\nStream Key: [hidden - reveal to copy]`);

    await revealButton.click();
    await expect(streamKeyInput).toHaveValue(streamKey);

    await copyObsButton.click();
    clipboardWrites = await page.evaluate(
      () => (window as typeof window & { __clipboardWrites: string[] }).__clipboardWrites
    );
    expect(clipboardWrites).toContain(`Service: Custom\nServer: ${ingestUrl}\nStream Key: ${streamKey}`);

    await copyViewerLinkButton.click();
    clipboardWrites = await page.evaluate(
      () => (window as typeof window & { __clipboardWrites: string[] }).__clipboardWrites
    );
    expect(clipboardWrites).toContain(`http://127.0.0.1:3000/channels/${channelId}`);

    await page.getByRole("button", { name: "Refresh now" }).click();
    await expect(
      page.getByTestId("test-stream-status-card").getByText("Live", { exact: true })
    ).toBeVisible();
    await expect(page.getByText("Preview is live. Check video and audio, then share.")).toBeVisible();
    await expect(page.getByLabel("Viewer link")).toHaveValue(`http://127.0.0.1:3000/channels/${channelId}`);
    await expect(page.getByRole("link", { name: "Open viewer" })).toHaveAttribute(
      "href",
      `http://127.0.0.1:3000/channels/${channelId}`
    );
  });
});
