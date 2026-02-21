import { expect, test } from "@playwright/test";

test.describe("creator uploads", () => {
  test("recovers from upload API errors and registers a VOD", async ({ page }) => {
    const channelId = "creator-uploads";

    await page.route("**/api/viewer/me", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          user: { id: "creator-uploads-user", displayName: "Uploader", roles: ["creator"] },
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
            ownerId: "creator-uploads-user",
            title: "Uploads Dashboard",
            category: "Talk Shows",
            tags: ["postshow"],
            liveState: "offline",
            createdAt: new Date("2024-06-01T10:00:00Z").toISOString(),
            updatedAt: new Date("2024-06-01T12:00:00Z").toISOString(),
          },
          owner: { id: "creator-uploads-user", displayName: "Uploader" },
          profile: { bio: "Testing upload flows", avatarUrl: undefined, bannerUrl: undefined },
          live: false,
          follow: { followers: 0, following: true },
          donationAddresses: [],
          subscription: { subscribers: 0, subscribed: true },
          playback: undefined,
          chat: { roomId: "uploads-room" },
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
    let getCalls = 0;

    await page.route("**/api/uploads**", async (route) => {
      const { method, url } = route.request();

      if (method === "GET") {
        getCalls += 1;
        if (getCalls === 1) {
          await route.fulfill({
            status: 500,
            contentType: "application/json",
            body: JSON.stringify({ message: "Upload API unavailable" }),
          });
          return;
        }
        await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(uploadItems) });
        return;
      }

      if (method === "POST") {
        const payload = route.request().postDataJSON();
        const newItem: UploadItem = {
          id: `upload-${uploadItems.length + 1}`,
          channelId,
          title: payload?.title ?? payload?.filename ?? "Untitled",
          filename: payload?.filename ?? "upload.bin",
          sizeBytes: payload?.sizeBytes ?? 0,
          status: "processing",
          progress: 18,
          createdAt: new Date("2024-06-01T12:30:00Z").toISOString(),
          updatedAt: new Date("2024-06-01T12:30:00Z").toISOString(),
          error: payload?.metadata?.fileLastModified ? undefined : "missing metadata",
        };
        uploadItems = [newItem];
        await route.fulfill({ status: 201, contentType: "application/json", body: JSON.stringify(newItem) });
        return;
      }

      if (method === "DELETE") {
        uploadItems = [];
        await route.fulfill({ status: 204, contentType: "application/json", body: "" });
        return;
      }

      await route.fulfill({ status: 404 });
    });

    const initialUploadsRequest = page.waitForResponse((response) => {
      const request = response.request();
      return (
        request.method() === "GET" &&
        response.status() === 500 &&
        response.url().includes(`/api/uploads?channelId=${channelId}`)
      );
    });

    await page.goto(`/creator/uploads/${channelId}`);
    await initialUploadsRequest;

    await expect(page.getByRole("heading", { level: 2, name: /manage uploads for uploads dashboard/i })).toBeVisible();
    const uploadActions = page.locator(".upload-actions");
    const uploadError = uploadActions.locator(".error");
    await expect(uploadError).toBeVisible();
    await expect(uploadError).toContainText(/upload api unavailable/i);

    await page.getByRole("button", { name: /refresh/i }).click();
    await expect(uploadError).toBeHidden();
    await expect(page.getByText(/no uploads yet/i)).toBeVisible();

    await page.getByLabel("Playback URL (optional)").fill("https://cdn.example.com/recap.m3u8");
    await page.getByLabel("Title").fill("Post-show recap");
    await page.getByLabel("Filename").fill("recap.mp4");
    await page.getByLabel("Size (bytes)").fill("1048576");

    await page.getByRole("button", { name: /add metadata/i }).click();
    const metadataRows = page.getByPlaceholder("Key");
    const metadataRowCount = await metadataRows.count();
    await metadataRows.nth(metadataRowCount - 1).fill("fileLastModified");
    const metadataValues = page.getByPlaceholder("Value");
    const metadataValueCount = await metadataValues.count();
    await metadataValues.nth(metadataValueCount - 1).fill(new Date("2024-06-01T11:00:00Z").toISOString());

    await page.getByRole("button", { name: /register upload/i }).click();

    await expect.poll(() => uploadItems.length).toBe(1);
    await expect(page.getByText(/processing · 18%/i)).toBeVisible();
    await expect(page.getByRole("button", { name: /delete/i })).toBeVisible();

    await page.getByRole("button", { name: /^ready$/i }).click();
    await expect(page.getByText(/no uploads match the selected filters/i)).toBeVisible();

    await page.getByRole("button", { name: /^all$/i }).click();
    await page.getByLabel(/search uploads/i).fill("recap");
    await expect(page.getByText(/processing · 18%/i)).toBeVisible();
    await page.getByLabel(/search uploads/i).fill("does-not-exist");
    await expect(page.getByText(/no uploads match the selected filters/i)).toBeVisible();
  });

  test("redirects guests back to uploads after sign-in", async ({ page }) => {
    const channelId = "creator-uploads-auth";
    let signedIn = false;

    await page.route("**/api/viewer/me", async (route) => {
      if (!signedIn) {
        await route.fulfill({
          status: 401,
          contentType: "application/json",
          body: JSON.stringify({ message: "not signed in", loginUrl: "/login" }),
        });
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          user: { id: "creator-uploads-user", displayName: "Uploader", roles: ["creator"] },
          loginUrl: "/login",
          logoutUrl: "/logout",
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
            ownerId: "creator-uploads-user",
            title: "Uploads Dashboard",
            category: "Talk Shows",
            tags: ["postshow"],
            liveState: "offline",
            createdAt: new Date("2024-06-01T10:00:00Z").toISOString(),
            updatedAt: new Date("2024-06-01T12:00:00Z").toISOString(),
          },
          owner: { id: "creator-uploads-user", displayName: "Uploader" },
          profile: { bio: "Testing upload flows", avatarUrl: undefined, bannerUrl: undefined },
          live: false,
          follow: { followers: 0, following: true },
          donationAddresses: [],
          subscription: { subscribers: 0, subscribed: true },
          playback: undefined,
          chat: { roomId: "uploads-room" },
        }),
      });
    });

    await page.route(`**/api/uploads?channelId=${channelId}`, async (route) => {
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify([]) });
    });

    await page.goto(`/creator/uploads/${channelId}?tab=ready`);
    await expect(page.getByText(/sign in to manage uploads/i)).toBeVisible();

    await page.waitForURL((url) => url.pathname === "/login" && url.searchParams.has("redirect"));
    const loginUrl = new URL(page.url());
    expect(loginUrl.searchParams.get("redirect")).toBe(`/creator/uploads/${channelId}?tab=ready`);
    expect(loginUrl.searchParams.get("redirect")).not.toContain(`/creator/live/${channelId}`);

    signedIn = true;
    const redirectTarget = loginUrl.searchParams.get("redirect")!;
    await page.goto(redirectTarget);

    await expect(page).toHaveURL(`/creator/uploads/${channelId}?tab=ready`);
    await expect(page.getByRole("heading", { level: 2, name: /manage uploads for uploads dashboard/i })).toBeVisible();
  });

});
