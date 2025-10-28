"use client";

import type { VodItem } from "../lib/viewer-api";

export function VodGallery({ items }: { items: VodItem[] }) {
  if (!items || items.length === 0) {
    return (
      <section className="surface stack">
        <h3>Past broadcasts</h3>
        <p className="muted">No VODs yet. Streams will show up here once the creator publishes replays.</p>
      </section>
    );
  }

  return (
    <section className="surface stack">
      <h3>Past broadcasts</h3>
      <ul className="vod-grid">
        {items.map((item) => {
          const minutes = Math.max(1, Math.round(item.durationSeconds / 60));
          const published = new Date(item.publishedAt).toLocaleDateString();
          return (
            <li key={item.id} className="vod-card">
              {item.thumbnailUrl && (
                <img src={item.thumbnailUrl} alt="" loading="lazy" />
              )}
              <div className="vod-card__body">
                <h4>{item.title}</h4>
                <p className="muted">
                  {minutes} minute{minutes === 1 ? "" : "s"} · {published}
                </p>
                {item.playbackUrl && (
                  <a className="secondary-button" href={item.playbackUrl} target="_blank" rel="noreferrer">
                    Watch replay
                  </a>
                )}
              </div>
            </li>
          );
        })}
      </ul>
    </section>
  );
}
