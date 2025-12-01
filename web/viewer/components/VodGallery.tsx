"use client";

import Image from "next/image";
import type { VodItem } from "../lib/viewer-api";

interface VodGalleryProps {
  items: VodItem[];
  error?: string;
  loading?: boolean;
}

export function VodGallery({ items, error, loading = false }: VodGalleryProps) {
  if (loading) {
    return (
      <section className="surface stack" aria-busy="true">
        <h3>Past broadcasts</h3>
        <p className="muted">Loading past broadcasts…</p>
        <div className="skeleton skeleton--text" aria-hidden="true" />
      </section>
    );
  }

  if (error) {
    return (
      <section className="surface stack" role="alert">
        <h3>Past broadcasts</h3>
        <p className="muted">We couldn&apos;t load past broadcasts right now.</p>
        <p className="muted">{error}</p>
      </section>
    );
  }

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
                <Image
                  src={item.thumbnailUrl}
                  alt=""
                  width={640}
                  height={360}
                  sizes="(min-width: 1024px) 33vw, 100vw"
                  style={{ width: "100%", height: "auto" }}
                />
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
