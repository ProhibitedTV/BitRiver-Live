import fs from "fs";
import path from "path";
import { injectAxe, checkA11y } from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

const styles = fs.readFileSync(path.join(__dirname, "../styles/globals.css"), "utf8");

test("channel directory markup has no major accessibility violations", async ({ page }) => {
  await page.setContent(`<!DOCTYPE html>
    <html lang="en">
      <head>
        <meta charset="utf-8" />
        <title>Directory Preview</title>
        <style>${styles}</style>
      </head>
      <body>
        <div id="app" class="container stack">
          <header class="stack" role="banner">
            <h1>Discover live channels</h1>
            <form class="search-bar" role="search" onsubmit="event.preventDefault(); window.lastQuery = this.elements.q.value;">
              <label for="q" class="sr-only">Search channels</label>
              <input id="q" name="q" type="search" placeholder="Search by channel, creator, or tag" aria-label="Search channels" />
              <button type="submit" class="secondary-button">Search</button>
            </form>
          </header>
          <main class="channel-layout" aria-live="polite">
            <section class="channel-layout__primary stack">
              <article class="surface stack" aria-label="Stream player placeholder">
                <h2>Stream offline</h2>
                <p class="muted">Follow to be notified when the broadcaster goes live.</p>
              </article>
              <section class="surface stack" aria-labelledby="vod-heading">
                <h2 id="vod-heading">Past broadcasts</h2>
                <ul class="vod-grid">
                  <li class="vod-card">
                    <div class="vod-card__body">
                      <h3>Retro Speedruns #5</h3>
                      <p class="muted">45 minutes · 10/20/2023</p>
                      <a href="#" class="secondary-button">Watch replay</a>
                    </div>
                  </li>
                </ul>
              </section>
            </section>
            <aside class="channel-layout__sidebar">
              <section class="chat-panel" aria-label="Live chat">
                <header class="chat-panel__header">
                  <h2>Live chat</h2>
                  <span class="muted">0 messages</span>
                </header>
                <div class="chat-panel__body">
                  <p class="muted">No messages yet. Be the first to say hello!</p>
                </div>
                <form class="chat-panel__form" aria-label="Send a chat message" onsubmit="event.preventDefault();">
                  <label for="chat-preview" class="sr-only">Chat message</label>
                  <textarea id="chat-preview" rows="3" placeholder="Share your thoughts"></textarea>
                  <button type="submit" class="primary-button">Send</button>
                </form>
              </section>
            </aside>
          </main>
        </div>
      </body>
    </html>`);

  await page.fill("input[type=search]", "retro");
  await page.click("button.secondary-button");
  const query = await page.evaluate(() => (window as any).lastQuery);
  expect(query).toBe("retro");

  await injectAxe(page);
  await checkA11y(page, "#app", {
    runOnly: {
      type: "tag",
      values: ["wcag2a", "wcag2aa"]
    }
  });
});
