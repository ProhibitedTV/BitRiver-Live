import { defineConfig, devices } from "@playwright/test";

const baseURL = process.env.PLAYWRIGHT_BASE_URL ?? "http://127.0.0.1:3000";
const compatibilitySpec = "**/compatibility.spec.ts";

export default defineConfig({
  testDir: "./tests",
  retries: 0,
  reporter: [["list"], ["html", { outputFolder: "playwright-report", open: "never" }]],
  use: {
    headless: true,
    baseURL
  },
  projects: [
    {
      name: "chromium-regression",
      testIgnore: compatibilitySpec,
      use: {
        ...devices["Desktop Chrome"],
        viewport: { width: 1280, height: 720 }
      }
    },
    {
      name: "chromium-compat",
      testMatch: compatibilitySpec,
      use: {
        ...devices["Desktop Chrome"],
        viewport: { width: 1280, height: 720 }
      }
    },
    {
      name: "firefox-compat",
      testMatch: compatibilitySpec,
      use: {
        ...devices["Desktop Firefox"],
        viewport: { width: 1280, height: 720 }
      }
    },
    {
      name: "webkit-compat",
      testMatch: compatibilitySpec,
      use: {
        ...devices["Desktop Safari"],
        viewport: { width: 1280, height: 720 }
      }
    },
    {
      name: "android-chrome-compat",
      testMatch: compatibilitySpec,
      use: {
        ...devices["Pixel 7"]
      }
    },
    {
      name: "iphone-webkit-compat",
      testMatch: compatibilitySpec,
      use: {
        ...devices["iPhone 15"]
      }
    }
  ],
  webServer: {
    command: "npm run start:test",
    url: baseURL,
    reuseExistingServer: !process.env.CI,
    timeout: 120_000
  }
});
