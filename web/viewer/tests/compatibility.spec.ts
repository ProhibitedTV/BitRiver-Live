import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";

import {
  authenticatedViewer,
  chatHistory,
  channelId,
  nextChatMessage,
  playbackResponse,
  unauthenticatedViewer,
  vodCollection
} from "./fixtures/channel";

const generatedAt = new Date("2026-01-01T00:00:00Z").toISOString();

async function mockSignedOutDirectory(page: Page) {
  await page.route("**/api/viewer/me", async (route) => {
    await route.fulfill({
      status: 401,
      contentType: "application/json",
      body: JSON.stringify({ allowSelfSignup: true })
    });
  });

  await page.route("**/api/directory**", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ channels: [], categories: [], generatedAt })
    });
  });
}

async function mockChannelApis(page: Page) {
  await page.route("**/api/viewer/me", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(authenticatedViewer)
    });
  });

  await page.route(`**/api/channels/${channelId}/playback`, async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(playbackResponse)
    });
  });

  await page.route(`**/api/channels/${channelId}/vods`, async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(vodCollection)
    });
  });
}

test.describe("release-critical viewer compatibility", () => {
  test("handles HLS capability, reads chat, and sends a message", async ({ page }) => {
    const sentMessages: string[] = [];
    await mockChannelApis(page);

    await page.route(`**/api/channels/${channelId}/chat**`, async (route) => {
      if (route.request().method() === "GET") {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify(chatHistory)
        });
        return;
      }

      const { content } = route.request().postDataJSON() as { content: string };
      sentMessages.push(content);
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(nextChatMessage(content, chatHistory.length + sentMessages.length))
      });
    });

    await page.goto(`/channels/${channelId}`);

    const video = page.locator("video");
    await expect(video).toBeVisible();
    await expect
      .poll(async () =>
        page.evaluate((expectedUrl) => {
          const element = document.querySelector("video");
          const source = element?.currentSrc ?? "";
          if (source.startsWith("blob:") || source === expectedUrl) {
            return "attached";
          }
          return document.body.textContent?.includes("Stream unavailable") ? "unsupported" : "pending";
        }, playbackResponse.playback?.playbackUrl)
      )
      .not.toBe("pending");

    const chatLog = page.getByRole("log");
    await expect(chatLog).toContainText("Welcome aboard the orbital maintenance stream!");

    const composer = page.getByRole("textbox", { name: "Chat message" });
    await composer.fill("Compatibility matrix check.");
    await page
      .getByRole("form", { name: "Send a chat message" })
      .getByRole("button", { name: "Send", exact: true })
      .click();

    await expect.poll(() => sentMessages[0]).toBe("Compatibility matrix check.");
    await expect(chatLog).toContainText("Compatibility matrix check.");
  });

  test("recovers when the playback API returns a transient failure", async ({ page }) => {
    let playbackAttempts = 0;
    let recover = false;

    await page.route("**/api/viewer/me", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(authenticatedViewer)
      });
    });

    await page.route(`**/api/channels/${channelId}/playback`, async (route) => {
      playbackAttempts += 1;
      if (!recover) {
        await route.fulfill({ status: 502, body: "upstream offline" });
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(playbackResponse)
      });
    });

    await page.route(`**/api/channels/${channelId}/vods`, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(vodCollection)
      });
    });

    await page.route(`**/api/channels/${channelId}/chat**`, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(chatHistory)
      });
    });

    await page.goto(`/channels/${channelId}`);

    const alert = page.getByTestId("channel-load-error");
    await expect(alert).toBeVisible();
    recover = true;
    await alert.getByRole("button", { name: "Try again" }).click();

    await expect(page.locator("video")).toBeVisible();
    await expect.poll(() => playbackAttempts).toBeGreaterThan(1);
  });

  test("keeps signed-out navigation, auth, and accessibility functional", async ({ page }) => {
    await mockSignedOutDirectory(page);
    await page.goto("/browse");

    await expect(page.getByRole("heading", { level: 1, name: /browse live channels/i })).toBeVisible();

    const accountActions = page.getByRole("group", { name: "Account and preferences" });
    const signIn = accountActions.getByRole("button", { name: "Sign in" });
    if (await signIn.isVisible()) {
      await signIn.click();
    } else {
      await page.getByRole("button", { name: "Open navigation menu" }).click();
      await page.locator("#viewer-nav-menu").getByRole("button", { name: "Sign in" }).click();
    }

    await expect(page.getByRole("dialog", { name: "Sign in to BitRiver Live" })).toBeVisible();
    await page.getByRole("button", { name: "Close", exact: true }).click();

    const results = await new AxeBuilder({ page })
      .include("main")
      .withTags(["wcag2a", "wcag2aa"])
      .analyze();
    expect(results.violations).toEqual([]);
  });

  test("loads the creator live setup with server-authored stream settings", async ({ page }) => {
    const creatorChannelId = "compat-creator";

    await page.route("**/api/viewer/me", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          user: {
            id: "compat-owner",
            displayName: "Compatibility Creator",
            email: "compat@example.com",
            roles: ["creator"]
          }
        })
      });
    });

    await page.route("**/api/channels", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify([
          {
            id: creatorChannelId,
            ownerId: "compat-owner",
            title: "Compatibility Creator",
            category: "Science & Tech",
            tags: ["compat"],
            liveState: "offline",
            createdAt: generatedAt,
            updatedAt: generatedAt,
            streamKey: "sk_compatibility_secret",
            ingestEndpoints: ["rtmp://ingest.example.com/live"]
          }
        ])
      });
    });

    await page.route(`**/api/channels/${creatorChannelId}/playback`, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          channel: {
            id: creatorChannelId,
            ownerId: "compat-owner",
            title: "Compatibility Creator",
            category: "Science & Tech",
            tags: ["compat"],
            liveState: "offline",
            createdAt: generatedAt,
            updatedAt: generatedAt
          },
          owner: { id: "compat-owner", displayName: "Compatibility Creator" },
          profile: {},
          live: false,
          follow: { followers: 0, following: false },
          donationAddresses: [],
          subscription: { subscribers: 0, subscribed: false },
          chat: { roomId: "compat-room" }
        })
      });
    });

    await page.route(`**/api/channels/${creatorChannelId}/sessions**`, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify([])
      });
    });

    await page.goto(`/creator/live/${creatorChannelId}`);

    await expect(page.getByRole("heading", { level: 2, name: /compatibility creator studio/i })).toBeVisible();
    await expect(page.getByLabel("Preferred ingest URL")).toHaveValue("rtmp://ingest.example.com/live");
    await expect(page.getByLabel("Stream key")).toHaveValue("********");
    await expect(page.getByRole("button", { name: "Reveal", exact: true })).toBeVisible();
  });

  test("keeps touch navigation and the auth overlay inside mobile viewports", async ({ page }, testInfo) => {
    const mobileProject = testInfo.project.name === "android-chrome-compat" || testInfo.project.name === "iphone-webkit-compat";
    test.skip(!mobileProject, "mobile-only compatibility assertion");

    await mockSignedOutDirectory(page);
    await page.goto("/browse");

    const navToggle = page.getByRole("button", { name: "Open navigation menu" });
    await expect(navToggle).toBeVisible();
    await navToggle.tap();
    await expect(page.locator("#viewer-nav-menu")).toBeVisible();

    const createAccount = page.locator("#viewer-nav-menu").getByRole("button", { name: "Create account" });
    await createAccount.tap();
    await expect(page.getByRole("dialog", { name: "Create your BitRiver account" })).toBeVisible();

    await expect
      .poll(async () =>
        page.evaluate(() => Math.max(document.documentElement.scrollWidth, document.body.scrollWidth) - window.innerWidth)
      )
      .toBeLessThanOrEqual(1);
  });

  test("shows the chat authentication fallback instead of a composer when unauthorized", async ({ page }) => {
    await page.route("**/api/viewer/me", async (route) => {
      await route.fulfill({
        status: 401,
        contentType: "application/json",
        body: JSON.stringify(unauthenticatedViewer)
      });
    });

    await page.route(`**/api/channels/${channelId}/playback`, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(playbackResponse)
      });
    });

    await page.route(`**/api/channels/${channelId}/vods`, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(vodCollection)
      });
    });

    await page.route(`**/api/channels/${channelId}/chat**`, async (route) => {
      await route.fulfill({
        status: 401,
        contentType: "application/json",
        body: JSON.stringify({ error: "authentication required" })
      });
    });

    await page.goto(`/channels/${channelId}`);

    await expect(page.getByText("Sign in to view and participate in chat.")).toBeVisible();
    await expect(page.getByRole("textbox", { name: /chat message/i })).toHaveCount(0);
  });
});
