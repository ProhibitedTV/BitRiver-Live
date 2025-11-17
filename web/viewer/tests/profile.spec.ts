import { expect, test } from "@playwright/test";

test.describe("profile page", () => {
  test("loads the viewer profile and saves edits", async ({ page }) => {
    await page.route("**/api/auth/session", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          user: { id: "viewer-1", displayName: "Viewer One", email: "viewer@example.com", roles: ["member"] },
        }),
      });
    });

    const profileResponse = {
      userId: "viewer-1",
      displayName: "Viewer One",
      bio: "Streaming sci-fi strategy games.",
      avatarUrl: "https://cdn.example.com/avatar.png",
      bannerUrl: "https://cdn.example.com/banner.jpg",
      featuredChannelId: undefined,
      topFriends: [],
      donationAddresses: [],
      channels: [],
      liveChannels: [],
      createdAt: new Date("2023-10-21T11:00:00Z").toISOString(),
      updatedAt: new Date("2023-10-21T12:00:00Z").toISOString(),
    };

    let lastUpdatePayload: any;
    await page.route("**/api/profiles/viewer-1", async (route) => {
      if (route.request().method() === "GET") {
        await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(profileResponse) });
        return;
      }
      const payload = route.request().postDataJSON();
      lastUpdatePayload = payload;
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ ...profileResponse, ...payload, updatedAt: new Date("2023-10-22T12:00:00Z").toISOString() }),
      });
    });

    await page.goto("/profile");

    await expect(page.getByRole("heading", { level: 1, name: "Profile" })).toBeVisible();
    await expect(page.getByLabel("Avatar URL")).toHaveValue(profileResponse.avatarUrl);
    await expect(page.getByLabel("Banner URL")).toHaveValue(profileResponse.bannerUrl);
    await expect(page.getByLabel("Bio")).toHaveValue(profileResponse.bio);

    await page.getByLabel("Bio").fill("Building a new stream schedule.");
    await page.getByLabel("Avatar URL").fill("https://cdn.example.com/new-avatar.png");
    await page.getByRole("button", { name: /save profile/i }).click();

    await expect(page.getByText(/profile saved/i)).toBeVisible();
    await expect.poll(() => lastUpdatePayload?.avatarUrl).toBe("https://cdn.example.com/new-avatar.png");
    await expect.poll(() => lastUpdatePayload?.bio).toBe("Building a new stream schedule.");
  });
});
