import { expect, test } from "@playwright/test";

const basePlayback = {
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
  messages: [
    {
      id: "msg-1",
      message: "Welcome to the stream!",
      sentAt: new Date("2023-10-21T12:00:00Z").toISOString(),
      user: {
        id: "owner-42",
        displayName: "DJ Nova",
        role: "host"
      }
    }
  ]
};

test.describe("channel route", () => {
  test("allows authenticated viewers to follow, subscribe, and chat", async ({ page }) => {
    await page.route("**/api/auth/session", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          user: {
            id: "viewer-1",
            displayName: "Viewer",
            email: "viewer@example.com",
            roles: ["member"]
          }
        })
      });
    });

    await page.route("**/api/channels/chan-42/playback", async (route) => {
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(basePlayback) });
    });

    await page.route("**/api/channels/chan-42/vods", async (route) => {
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ channelId: "chan-42", items: [] }) });
    });

    let lastPostedMessage: string | undefined;
    await page.route("**/api/channels/chan-42/chat", async (route) => {
      if (route.request().method() === "GET") {
        await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(chatTranscript) });
        return;
      }
      const body = route.request().postDataJSON() as { message: string };
      lastPostedMessage = body.message;
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          id: "msg-2",
          message: body.message,
          sentAt: new Date("2023-10-21T12:05:00Z").toISOString(),
          user: {
            id: "viewer-1",
            displayName: "Viewer",
            role: "member"
          }
        })
      });
    });

    let followCalls = 0;
    await page.route("**/api/channels/chan-42/follow", async (route) => {
      followCalls += 1;
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ followers: 11, following: true })
      });
    });

    let subscribeCalls = 0;
    await page.route("**/api/channels/chan-42/subscribe", async (route) => {
      subscribeCalls += 1;
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ subscribers: 4, subscribed: true, tier: "Plus" })
      });
    });

    await page.goto("/channels/chan-42");

    await expect(page.getByRole("heading", { level: 1, name: "Deep Space Beats" })).toBeVisible();
    await expect(page.getByText(/welcome to the stream/i)).toBeVisible();

    await page.getByRole("button", { name: /follow · 10 supporters/i }).click();
    await expect(page.getByRole("button", { name: /following · 11 supporters/i })).toBeVisible();
    await expect.poll(() => followCalls).toBeGreaterThan(0);

    await page.getByRole("button", { name: /subscribe/i }).click();
    await expect(page.getByRole("button", { name: /subscribed · plus/i })).toBeVisible();
    await expect.poll(() => subscribeCalls).toBeGreaterThan(0);

    const chatInput = page.getByRole("textbox", { name: /chat message/i });
    await chatInput.fill("Hello from viewer");
    await page.getByRole("button", { name: "Send" }).click();

    await expect.poll(() => lastPostedMessage).toBe("Hello from viewer");
    await expect(page.getByText("Hello from viewer")).toBeVisible();
  });

  test("prompts viewers to authenticate when required", async ({ page }) => {
    await page.route("**/api/auth/session", async (route) => {
      await route.fulfill({ status: 401, body: "Unauthorized" });
    });

    await page.route("**/api/channels/chan-42/playback", async (route) => {
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(basePlayback) });
    });

    await page.route("**/api/channels/chan-42/vods", async (route) => {
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ channelId: "chan-42", items: [] }) });
    });

    await page.route("**/api/channels/chan-42/chat", async (route) => {
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(chatTranscript) });
    });

    let followAttempted = false;
    await page.route("**/api/channels/chan-42/follow", async (route) => {
      followAttempted = true;
      await route.fulfill({ status: 403, body: "Forbidden" });
    });

    await page.goto("/channels/chan-42");

    const followButton = page.getByRole("button", { name: /follow · 10 supporters/i });
    await followButton.click();
    await expect(page.getByText(/sign in from the header to follow this channel/i)).toBeVisible();
    await expect.poll(() => followAttempted).toBe(false);

    const textarea = page.getByRole("textbox", { name: /chat message/i });
    await expect(textarea).toBeDisabled();
    await expect(page.getByRole("button", { name: "Send" })).toBeDisabled();
  });
});

test.describe("authentication controls", () => {
  test("navbar login flow authenticates and logs out via mocked APIs", async ({ page }) => {
    let loginPayload: { email: string; password: string } | undefined;
    let logoutCalled = false;

    await page.route("**/api/auth/session", async (route) => {
      if (route.request().method() === "DELETE") {
        logoutCalled = true;
        await route.fulfill({ status: 204 });
        return;
      }
      await route.fulfill({ status: 401, body: "Unauthorized" });
    });

    await page.route("**/api/auth/login", async (route) => {
      loginPayload = route.request().postDataJSON() as { email: string; password: string };
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          user: {
            id: "viewer-1",
            displayName: "Viewer",
            email: loginPayload.email,
            roles: ["member"]
          }
        })
      });
    });

    await page.goto("/");

    await page.getByRole("button", { name: "Sign in" }).click();
    await page.getByLabel("Email").fill("viewer@example.com");
    await page.getByLabel("Password").fill("hunter2!!");
    await page.getByRole("button", { name: "Sign in", exact: true }).click();

    await expect.poll(() => loginPayload).toBeTruthy();
    await expect(page.getByText(/signed in as viewer/i)).toBeVisible();

    await page.getByRole("button", { name: "Sign out" }).click();
    await expect.poll(() => logoutCalled).toBe(true);
    await expect(page.getByRole("button", { name: "Sign in" })).toBeVisible();
  });

  test("theme toggle updates the rendered document", async ({ page }) => {
    await page.route("**/api/auth/session", async (route) => {
      await route.fulfill({ status: 401, body: "Unauthorized" });
    });

    await page.route("**/api/directory**", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ channels: [], generatedAt: new Date().toISOString() })
      });
    });

    await page.goto("/");

    const toggle = page.getByRole("button", { name: /switch to light theme/i });
    await expect(page.locator("body")).not.toHaveAttribute("data-theme", "light");

    await toggle.click();
    await expect(page.locator("body")).toHaveAttribute("data-theme", "light");
    await expect(toggle).toHaveAttribute("aria-label", /switch to dark theme/i);

    await toggle.click();
    await expect(page.locator("body")).not.toHaveAttribute("data-theme", "light");
  });
});
