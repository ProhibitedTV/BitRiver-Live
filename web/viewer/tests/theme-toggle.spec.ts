import fs from "fs";
import path from "path";
import { expect, test } from "@playwright/test";

const styles = fs.readFileSync(path.join(__dirname, "../styles/globals.css"), "utf8");

test("theme toggle updates the body data-theme attribute", async ({ page }) => {
  await page.setContent(`<!DOCTYPE html>
    <html lang="en">
      <head>
        <meta charset="utf-8" />
        <title>Theme toggle preview</title>
        <style>${styles}</style>
      </head>
      <body>
        <button id="toggle" class="secondary-button" type="button"></button>
        <script>
          const button = document.getElementById('toggle');
          let theme = 'dark';
          const applyTheme = () => {
            if (theme === 'light') {
              document.body.setAttribute('data-theme', 'light');
              button.textContent = '🌙 Dark';
              button.setAttribute('aria-label', 'Switch to dark theme');
            } else {
              document.body.removeAttribute('data-theme');
              button.textContent = '🌞 Light';
              button.setAttribute('aria-label', 'Switch to light theme');
            }
            window.currentTheme = theme;
          };
          button.addEventListener('click', () => {
            theme = theme === 'light' ? 'dark' : 'light';
            applyTheme();
          });
          applyTheme();
        </script>
      </body>
    </html>`);

  await expect(page.locator("body")).not.toHaveAttribute("data-theme", "light");
  await expect(page.locator("#toggle")).toHaveText("🌞 Light");

  await page.click("#toggle");
  await expect(page.locator("body")).toHaveAttribute("data-theme", "light");
  await expect(page.locator("#toggle")).toHaveText("🌙 Dark");

  await page.click("#toggle");
  await expect(page.locator("body")).not.toHaveAttribute("data-theme", "light");
  const theme = await page.evaluate(() => (window as any).currentTheme);
  expect(theme).toBe("dark");
});
