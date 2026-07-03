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
    updatedAt: new Date("2023-10-21T11:00:00Z").toISOString(),
  },
  owner: {
    id: "owner-42",
    displayName: "DJ Nova",
  },
  profile: {
    bio: "Streaming vinyl sets from a solar-powered cabin.",
    avatarUrl: undefined,
    bannerUrl: undefined,
  },
  live: true,
  follow: {
    followers: 10,
    following: false,
  },
  donationAddresses: [
    { currency: "eth", address: "0xabc123", note: "Main" },
    { currency: "btc", address: "bc1xyz" },
  ],
  subscription: {
    subscribers: 3,
    subscribed: false,
  },
  playback: undefined,
  chat: {
    roomId: "room-1",
  },
};

const chatTranscript = [
  {
    id: "msg-1",
    userId: "owner-42",
    channelId: "chan-42",
    content: "Welcome to the stream!",
    createdAt: new Date("2023-10-21T12:00:00Z").toISOString(),
  },
];

test.describe("channel route", () => {
  test("allows authenticated viewers to follow, subscribe, and chat", async ({ page }) => {
    await page.route("**/api/viewer/me", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          user: {
            id: "viewer-1",
            displayName: "Viewer",
            email: "viewer@example.com",
            roles: ["member"],
          },
          loginUrl: "https://auth.example.com/login",
          logoutUrl: "https://auth.example.com/logout",
        }),
      });
    });

    await page.route("**/api/channels/chan-42/playback", async (route) => {
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(basePlayback) });
    });

    await page.route("**/api/channels/chan-42/vods", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ channelId: "chan-42", items: [] })
      });
    });

    let lastPostedMessage: string | undefined;
    await page.route("**/api/channels/chan-42/chat**", async (route) => {
      if (route.request().method() === "GET") {
        await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(chatTranscript) });
        return;
      }
      const body = route.request().postDataJSON() as { content: string };
      lastPostedMessage = body.content;
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          id: "msg-2",
          channelId: "chan-42",
          userId: "viewer-1",
          content: body.content,
          createdAt: new Date("2023-10-21T12:05:00Z").toISOString()
        })
      });
    });

    let followCalls = 0;
    await page.route("**/api/channels/chan-42/follow", async (route) => {
      followCalls += 1;
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ followers: 11, following: true }),
      });
    });

    let subscribeCalls = 0;
    await page.route("**/api/channels/chan-42/subscribe", async (route) => {
      subscribeCalls += 1;
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ subscribers: 4, subscribed: true, tier: "Plus" }),
      });
    });

    await page.route("**/api/channels/chan-42/monetization/tips", async (route) => {
      const body = route.request().postDataJSON() as {
        amount: number;
        currency: string;
        provider: string;
        reference: string;
        walletAddress?: string;
        message?: string;
      };
      await route.fulfill({
        status: 201,
        contentType: "application/json",
        body: JSON.stringify({
          id: "tip-1",
          channelId: "chan-42",
          fromUserId: "viewer-1",
          amount: body?.amount ?? 0,
          currency: body?.currency ?? "ETH",
          provider: body?.provider ?? "viewer",
          reference: body?.reference ?? "",
          walletAddress: body?.walletAddress ?? null,
          message: body?.message ?? null,
          createdAt: new Date("2023-10-21T12:10:00Z").toISOString(),
        }),
      });
    });

    await page.goto("/channels/chan-42");

    await expect(page.getByRole("heading", { level: 1, name: "Deep Space Beats" })).toBeVisible();
    await expect(page.getByText(/live now/i)).toBeVisible();
    await expect(page.getByRole("heading", { name: "Manage this channel" })).toHaveCount(0);

    await page.getByRole("button", { name: /follow - 10 followers/i }).click();
    await expect(page.getByRole("button", { name: /following - 11 followers/i })).toBeVisible();
    await expect.poll(() => followCalls).toBeGreaterThan(0);

    await page.getByRole("button", { name: /subscribe/i }).click();
    await expect(page.getByRole("button", { name: /subscribed - plus/i })).toBeVisible();
    await expect.poll(() => subscribeCalls).toBeGreaterThan(0);

    const chatInput = page.getByRole("textbox", { name: /chat message/i });
    await chatInput.fill("Hello from viewer");
    await page
      .getByRole("form", { name: "Send a chat message" })
      .getByRole("button", { name: "Send", exact: true })
      .click();

    await expect.poll(() => lastPostedMessage).toBe("Hello from viewer");
    await expect(page.getByText("Hello from viewer")).toBeVisible();

    await page.getByRole("button", { name: /send a tip/i }).click();
    const tipDialog = page.getByRole("dialog", { name: /send a tip/i });
    const amountInput = tipDialog.getByLabel("Amount");
    await expect(amountInput).toBeVisible();
    await expect(amountInput).toHaveValue("");
    await amountInput.fill("0.0005");
    await tipDialog.getByLabel("Currency").selectOption({ label: "BTC" });
    await tipDialog.getByLabel("Wallet reference").fill("txn-77");
    await tipDialog.getByLabel("Message (optional)").fill("Great vibes!");
    await expect(tipDialog.getByRole("alert")).toHaveCount(0);
    await expect(tipDialog.getByRole("button", { name: /send tip/i })).toBeEnabled();
    await tipDialog.getByRole("button", { name: "Cancel", exact: true }).click();
    await expect(tipDialog).toBeHidden();
  });

  test("prompts viewers to authenticate when required", async ({ page }) => {
    await page.route("**/api/viewer/me", async (route) => {
      await route.fulfill({
        status: 401,
        contentType: "application/json",
        body: JSON.stringify({ loginUrl: "/login" }),
      });
    });

    await page.route("**/api/channels/chan-42/playback", async (route) => {
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(basePlayback) });
    });

    await page.route("**/api/channels/chan-42/vods", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ channelId: "chan-42", items: [] })
      });
    });

    await page.route("**/api/channels/chan-42/chat**", async (route) => {
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(chatTranscript) });
    });

    let followAttempted = false;
    await page.route("**/api/channels/chan-42/follow", async (route) => {
      followAttempted = true;
      await route.fulfill({ status: 403, body: "Forbidden" });
    });

    await page.goto("/channels/chan-42");

    await expect(page.getByRole("heading", { name: "Manage this channel" })).toHaveCount(0);
    await expect(page.getByText("Sign in to view and participate in chat.")).toBeVisible();
    await expect(page.getByRole("button", { name: "Sign in to chat" })).toBeVisible();
    await expect(page.getByRole("textbox", { name: /chat message/i })).toHaveCount(0);
    await expect(page.getByRole("form", { name: "Send a chat message" })).toHaveCount(0);

    const tipButton = page.getByRole("button", { name: /send a tip/i });
    await tipButton.click();
    await expect(page.getByText(/sign in from the header to send a tip/i)).toBeVisible();

    const followButton = page.getByRole("button", { name: /follow - 10 followers/i });
    await followButton.click();
    await expect(page).toHaveURL(/\/login\?redirect=%2Fchannels%2Fchan-42$/);
    await expect.poll(() => followAttempted).toBe(false);
  });

  test("gives channel owners compact studio actions from the public channel", async ({ page }) => {
    await page.route("**/api/viewer/me", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          user: {
            id: "owner-42",
            displayName: "DJ Nova",
            email: "nova@example.com",
            roles: ["creator"],
          },
        }),
      });
    });

    await page.route("**/api/channels/chan-42/playback", async (route) => {
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(basePlayback) });
    });

    await page.route("**/api/channels/chan-42/vods", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ channelId: "chan-42", items: [] })
      });
    });

    await page.route("**/api/channels/chan-42/chat**", async (route) => {
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(chatTranscript) });
    });

    await page.goto("/channels/chan-42");

    await expect(page.getByRole("heading", { level: 1, name: "Deep Space Beats" })).toBeVisible();
    await expect(page.getByText(/owner view/i)).toBeVisible();
    await expect(page.getByRole("heading", { name: "Manage this channel" })).toBeVisible();
    const studioNav = page.getByRole("navigation", { name: /deep space beats channel tools/i });
    await expect(studioNav.getByRole("link", { name: "Public preview" })).toHaveAttribute("href", "/channels/chan-42");
    await expect(studioNav.getByRole("link", { name: "Go live" })).toHaveAttribute("href", "/creator/live/chan-42");
    await expect(studioNav.getByRole("link", { name: "Uploads" })).toHaveAttribute("href", "/creator/uploads/chan-42");
    await expect(studioNav.getByRole("link", { name: "Schedule" })).toHaveAttribute("href", "/creator/live/chan-42#channel-schedule");
    await expect(studioNav.getByRole("link", { name: "Share link" })).toHaveAttribute("href", "/creator/live/chan-42#channel-share");
  });

  test("keeps mobile watch navigation focused on video, chat, and details", async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });

    await page.route("**/api/viewer/me", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          user: {
            id: "viewer-1",
            displayName: "Viewer",
            email: "viewer@example.com",
            roles: ["member"],
          },
        }),
      });
    });

    await page.route("**/api/channels/chan-42/playback", async (route) => {
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(basePlayback) });
    });

    await page.route("**/api/channels/chan-42/vods", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ channelId: "chan-42", items: [] })
      });
    });

    await page.route("**/api/channels/chan-42/chat**", async (route) => {
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(chatTranscript) });
    });

    await page.goto("/channels/chan-42");

    await expect(page.locator(".viewer-shell")).toHaveClass(/viewer-shell--following-disabled/);
    await expect(page.getByRole("button", { name: "Show following" })).toHaveCount(0);

    const player = page.locator(".channel-player");
    const chat = page.locator("#channel-chat");
    await expect(player).toBeVisible();
    await expect(chat).toBeVisible();
    const playerBox = await player.boundingBox();
    const chatBox = await chat.boundingBox();
    expect(playerBox).not.toBeNull();
    expect(chatBox).not.toBeNull();
    expect(playerBox?.y ?? 0).toBeLessThan(chatBox?.y ?? 0);

    const watchNav = page.getByRole("navigation", { name: "Watch page sections" });
    await expect(watchNav.getByRole("link", { name: "Chat" })).toBeVisible();
    await expect(watchNav.getByRole("link", { name: "Details" })).toBeVisible();
    await expect(watchNav.getByRole("button", { name: "Videos" })).toBeVisible();

    await watchNav.getByRole("link", { name: "Chat" }).click();
    await expect(chat).toBeInViewport();

    await watchNav.getByRole("button", { name: "Videos" }).click();
    await expect(page.getByRole("tab", { name: "Videos" })).toHaveAttribute("aria-selected", "true");
  });

  test("surfaces tip errors when submission fails", async ({ page }) => {
    await page.route("**/api/viewer/me", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          user: {
            id: "viewer-1",
            displayName: "Viewer",
            email: "viewer@example.com",
            roles: ["member"],
          },
          loginUrl: "https://auth.example.com/login",
          logoutUrl: "https://auth.example.com/logout",
        }),
      });
    });

    await page.route("**/api/channels/chan-42/playback", async (route) => {
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(basePlayback) });
    });

    await page.route("**/api/channels/chan-42/vods", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ channelId: "chan-42", items: [] })
      });
    });

    await page.route("**/api/channels/chan-42/chat**", async (route) => {
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(chatTranscript) });
    });

    await page.route("**/api/channels/chan-42/monetization/tips", async (route) => {
      await route.fulfill({ status: 422, body: "Invalid reference" });
    });

    await page.goto("/channels/chan-42");

    await page.getByRole("button", { name: /send a tip/i }).click();
    const tipDialog = page.getByRole("dialog", { name: /send a tip/i });
    await tipDialog.getByLabel("Amount").fill("0.0005");
    await tipDialog.getByLabel("Wallet reference").fill("bad-ref");
    await tipDialog.getByRole("button", { name: /send tip/i }).click();

    await expect(tipDialog.getByText(/invalid reference/i)).toBeVisible();
    await expect(tipDialog).toBeVisible();
  });
});

test.describe("authentication controls", () => {
  test("signed-out navbar actions open the in-viewer auth flow when no external login URL is configured", async ({
    page
  }) => {
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
        body: JSON.stringify({ channels: [], generatedAt: new Date().toISOString() })
      });
    });

    await page.goto("/");

    const accountActions = page.getByRole("group", { name: "Account and preferences" });

    await accountActions.getByRole("button", { name: "Sign in" }).click();
    const signInDialog = page.getByRole("dialog", { name: "Sign in to BitRiver Live" });
    await expect(signInDialog).toBeVisible();

    await signInDialog.getByRole("button", { name: "Close" }).click();

    await accountActions.getByRole("button", { name: "Create account" }).click();
    await expect(page.getByRole("dialog", { name: "Create your BitRiver account" })).toBeVisible();
  });

  test("navbar sign-in button redirects to the configured login URL", async ({ page }) => {
    await page.route("**/api/viewer/me", async (route) => {
      await route.fulfill({
        status: 401,
        contentType: "application/json",
        body: JSON.stringify({ loginUrl: "/login" }),
      });
    });

    await page.goto("/");

    await page.getByRole("group", { name: "Account and preferences" }).getByRole("button", { name: "Sign in" }).click();

    await expect(page).toHaveURL(/\/login\?redirect=%2F$/);
  });

  test("navbar sign-out clears the viewer session", async ({ page }) => {
    let signedIn = true;
    let logoutCalled = false;

    await page.route("**/api/viewer/me", async (route) => {
      if (route.request().method() === "DELETE") {
        logoutCalled = true;
        signedIn = false;
        await route.fulfill({ status: 204 });
        return;
      }

      const body = signedIn
        ? {
            user: {
              id: "viewer-1",
              displayName: "Viewer",
              email: "viewer@example.com",
              roles: ["member"],
            },
            loginUrl: "/login",
            logoutUrl: "/logout",
          }
        : { loginUrl: "/login", logoutUrl: "/logout" };

      await route.fulfill({
        status: signedIn ? 200 : 401,
        contentType: "application/json",
        body: JSON.stringify(body),
      });
    });

    await page.route("**/logout", async (route) => {
      logoutCalled = true;
      signedIn = false;
      await route.fulfill({ status: 204 });
    });

    await page.goto("/");

    await page.getByRole("button", { name: "Open account menu" }).click();
    await page.getByRole("button", { name: "Sign out" }).click();

    await expect.poll(() => logoutCalled).toBe(true);
    await expect(
      page.getByRole("group", { name: "Account and preferences" }).getByRole("button", { name: "Sign in" })
    ).toBeVisible();
  });

  test("theme toggle updates the rendered document", async ({ page }) => {
    await page.addInitScript(() => {
      window.localStorage.setItem("viewer-theme", "dark");
    });

    await page.route("**/api/viewer/me", async (route) => {
      await route.fulfill({ status: 401, body: "Unauthorized" });
    });

    await page.route("**/api/directory**", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ channels: [], generatedAt: new Date().toISOString() }),
      });
    });

    await page.goto("/");

    const siteMenu = page.getByRole("button", { name: "Open site menu" });
    const body = page.locator("body");
    await expect(body).not.toHaveAttribute("data-theme", "light");

    await siteMenu.click();
    let toggle = page.locator("#viewer-user-menu").getByRole("button", { name: /switch to light theme/i });
    await expect(toggle).toBeVisible();
    await toggle.click();
    await expect(body).toHaveAttribute("data-theme", "light");

    await siteMenu.click();
    toggle = page.locator("#viewer-user-menu").getByRole("button", { name: /switch to dark theme/i });
    await expect(toggle).toBeVisible();

    await toggle.click();
    await expect(body).not.toHaveAttribute("data-theme", "light");
  });
});
