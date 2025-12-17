import { expect, test } from "@playwright/test";

import {
  authenticatedViewer,
  chatHistory,
  channelId,
  nextChatMessage,
  playbackResponse,
  unauthenticatedViewer,
  vodCollection
} from "./fixtures/channel";

test.describe("channel playback and chat integration", () => {
  test("renders HLS playback and chat history, then records new messages", async ({ page }) => {
    const sentMessages: string[] = [];

    await page.route("**/api/viewer/me", async (route) => {
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(authenticatedViewer) });
    });

    await page.route(`**/api/channels/${channelId}/playback`, async (route) => {
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(playbackResponse) });
    });

    await page.route(`**/api/channels/${channelId}/vods`, async (route) => {
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(vodCollection) });
    });

    await page.route(`**/api/channels/${channelId}/chat**`, async (route) => {
      if (route.request().method() === "GET") {
        await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(chatHistory) });
        return;
      }

      const { content } = route.request().postDataJSON() as { userId: string; content: string };
      sentMessages.push(content);
      const reply = nextChatMessage(content, chatHistory.length + sentMessages.length);
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(reply) });
    });

    await page.goto(`/channels/${channelId}`);

    const video = page.locator("video");
    await expect(video).toBeVisible();
    await expect(video).toHaveAttribute("src", playbackResponse.playback?.playbackUrl ?? "");
    await expect(video).toHaveAttribute("poster", playbackResponse.playback?.originUrl ?? "");

    const chatLog = page.getByRole("log");
    await expect(chatLog).toContainText("Welcome aboard the orbital maintenance stream!");
    await expect(page.getByText("2 messages")).toBeVisible();

    const composer = page.getByLabel("Chat message");
    await composer.fill("Copy that, checking tether.");
    await page.getByRole("button", { name: "Send" }).click();

    await expect.poll(() => sentMessages[0]).toBe("Copy that, checking tether.");
    await expect(chatLog).toContainText("Copy that, checking tether.");
    await expect(page.getByText("3 messages")).toBeVisible();
  });

  test("offers retry when playback API fails then recovers", async ({ page }) => {
    let playbackAttempts = 0;

    await page.route("**/api/viewer/me", async (route) => {
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(authenticatedViewer) });
    });

    await page.route(`**/api/channels/${channelId}/playback`, async (route) => {
      playbackAttempts += 1;
      if (playbackAttempts === 1) {
        await route.fulfill({ status: 502, body: "upstream offline" });
        return;
      }
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(playbackResponse) });
    });

    await page.route(`**/api/channels/${channelId}/vods`, async (route) => {
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(vodCollection) });
    });

    await page.route(`**/api/channels/${channelId}/chat**`, async (route) => {
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(chatHistory) });
    });

    await page.goto(`/channels/${channelId}`);

    await expect(page.getByRole("alert")).toContainText("couldn't load this channel");
    await page.getByRole("button", { name: "Try again" }).click();

    await expect(page.getByRole("heading", { level: 1, name: playbackResponse.channel.title })).toBeVisible();
    await expect(page.locator("video")).toBeVisible();
    await expect.poll(() => playbackAttempts).toBe(2);
  });

  test("prompts chat authentication when chat API rejects access", async ({ page }) => {
    await page.route("**/api/viewer/me", async (route) => {
      await route.fulfill({ status: 401, contentType: "application/json", body: JSON.stringify(unauthenticatedViewer) });
    });

    await page.route(`**/api/channels/${channelId}/playback`, async (route) => {
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(playbackResponse) });
    });

    await page.route(`**/api/channels/${channelId}/vods`, async (route) => {
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(vodCollection) });
    });

    await page.route(`**/api/channels/${channelId}/chat**`, async (route) => {
      await route.fulfill({ status: 401, contentType: "application/json", body: JSON.stringify({ error: "authentication required" }) });
    });

    await page.goto(`/channels/${channelId}`);

    await expect(page.getByText("Sign in to view and participate in chat.")).toBeVisible();
    await expect(page.getByRole("textbox", { name: /chat message/i })).toBeDisabled();
    await expect(page.getByRole("button", { name: "Send" })).toBeDisabled();
  });
});
