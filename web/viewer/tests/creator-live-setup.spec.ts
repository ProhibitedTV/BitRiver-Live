import { expect, test } from "@playwright/test";

test.describe("creator live setup", () => {
  test("reveals stream key and supports copy actions", async ({ page }) => {
    const channelId = "creator-live-setup";
    const streamKey = "sk_live_setup_123";
    const ingestUrl = "rtmp://ingest.example.com/live";

    await page.addInitScript(() => {
      const clipboardWrites: string[] = [];
      Object.defineProperty(window, "__clipboardWrites", {
        value: clipboardWrites,
        writable: false,
      });
      Object.defineProperty(navigator, "clipboard", {
        value: {
          writeText: async (text: string) => {
            clipboardWrites.push(text);
          },
        },
        configurable: true,
      });
    });

    await page.route("**/api/viewer/me", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          user: { id: "creator-live-owner", displayName: "Live Owner", email: "owner@example.com", roles: ["creator"] },
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
            ownerId: "creator-live-owner",
            title: "Creator Live Setup",
            category: "Science & Tech",
            tags: ["setup"],
            liveState: "offline",
            createdAt: new Date("2024-05-01T10:00:00Z").toISOString(),
            updatedAt: new Date("2024-05-01T10:30:00Z").toISOString(),
          },
          owner: { id: "creator-live-owner", displayName: "Live Owner" },
          profile: { bio: "Creator setup", avatarUrl: undefined, bannerUrl: undefined },
          live: false,
          follow: { followers: 0, following: false },
          donationAddresses: [],
          subscription: { subscribers: 0, subscribed: false },
          playback: undefined,
          chat: { roomId: "room-live-setup" },
        }),
      });
    });

    await page.route(`**/api/channels/${channelId}/sessions`, async (route) => {
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify([]) });
    });

    await page.route(`**/api/channels/${channelId}/sessions/status`, async (route) => {
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ status: "idle" }) });
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
            ingestEndpoints: [ingestUrl, "rtmp://backup.example.com/live"],
          },
        ]),
      });
    });

    await page.goto(`/creator/live/${channelId}`);

    const streamKeyInput = page.getByLabel("Stream key");
    const revealButton = page.getByRole("button", { name: "Reveal", exact: true });
    const copyKeyButton = page.getByRole("button", { name: "Copy key", exact: true });
    const copyIngestButton = page.getByTestId("copy-preferred-ingest-endpoint");
    const copyObsButton = page.getByTestId("copy-obs-settings");

    await expect(streamKeyInput).toHaveValue("••••••••");
    await expect(copyObsButton).toBeVisible();

    await expect(copyKeyButton).toBeEnabled();
    await copyKeyButton.click();
    await expect(page.getByText("Copied", { exact: true })).toBeVisible();

    let clipboardWrites = await page.evaluate(() => (window as typeof window & { __clipboardWrites: string[] }).__clipboardWrites);
    expect(clipboardWrites).toContain(streamKey);

    await copyIngestButton.click();
    await expect(copyIngestButton).toHaveText("Copied");

    clipboardWrites = await page.evaluate(() => (window as typeof window & { __clipboardWrites: string[] }).__clipboardWrites);
    expect(clipboardWrites).toContain(ingestUrl);

    await copyObsButton.click();
    await expect(page.getByText("Copied OBS settings", { exact: true })).toBeVisible();

    clipboardWrites = await page.evaluate(() => (window as typeof window & { __clipboardWrites: string[] }).__clipboardWrites);
    expect(clipboardWrites).toContain(`Server: ${ingestUrl}\nStream Key: [hidden - reveal to copy]`);

    await revealButton.click();
    await expect(streamKeyInput).toHaveValue(streamKey);

    await copyObsButton.click();
    clipboardWrites = await page.evaluate(() => (window as typeof window & { __clipboardWrites: string[] }).__clipboardWrites);
    expect(clipboardWrites).toContain(`Server: ${ingestUrl}\nStream Key: ${streamKey}`);
  });
});
