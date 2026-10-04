import { test as base } from "@playwright/test";

// These tests prove real player source attachment, not media decoding. Keep the
// metadata-only fixture's manifest pending so external DNS/HTTP timing cannot
// destroy hls.js before the attachment assertion. Failure tests override this
// route with an explicit response; the real-media golden path owns decoding.
export const test = base.extend({
  page: async ({ page }, runTest) => {
    let releaseManifest!: () => void;
    const manifestPending = new Promise<void>((resolve) => { releaseManifest = resolve; });
    await page.route("https://cdn.example.com/**/*.m3u8", async (route) => {
      await manifestPending;
      if (!page.isClosed()) {
        await route.abort();
      }
    });
    try {
      await runTest(page);
    } finally {
      releaseManifest();
      await page.unrouteAll({ behavior: "wait" });
    }
  }
});
