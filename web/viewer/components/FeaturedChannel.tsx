"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import type { DirectoryChannel } from "../lib/viewer-api";

interface FeaturedChannelProps {
  channels?: DirectoryChannel[];
  loading?: boolean;
  autoPlay?: boolean;
  autoPlayIntervalMs?: number;
}

export function FeaturedChannel({
  channels = [],
  loading = false,
  autoPlay = true,
  autoPlayIntervalMs = 8000,
}: FeaturedChannelProps) {
  const slides = useMemo(() => channels.filter(Boolean), [channels]);
  const [activeIndex, setActiveIndex] = useState(0);
  const [autoPlayEnabled, setAutoPlayEnabled] = useState(autoPlay);

  useEffect(() => {
    setAutoPlayEnabled(autoPlay);
  }, [autoPlay]);

  useEffect(() => {
    setActiveIndex(0);
  }, [slides.length]);

  useEffect(() => {
    if (!autoPlayEnabled || slides.length <= 1) {
      return undefined;
    }

    const timer = window.setInterval(() => {
      setActiveIndex((current) => (current + 1) % slides.length);
    }, autoPlayIntervalMs);

    return () => window.clearInterval(timer);
  }, [autoPlayEnabled, slides.length, autoPlayIntervalMs]);

  if (loading) {
    return (
      <div className="featured-channel surface" aria-busy="true" aria-live="polite">
        <div className="muted">Loading featured channels…</div>
      </div>
    );
  }

  if (!slides.length) {
    return (
      <div className="featured-channel surface" role="region" aria-label="Featured channels">
        <div className="featured-channel__content">
          <span className="featured-channel__eyebrow muted">Featured</span>
          <h2>No featured broadcast yet</h2>
          <p className="muted">
            Once BitRiver highlights a standout creator, their stream will appear here so you can jump in instantly.
          </p>
        </div>
      </div>
    );
  }

  const activeChannel = slides[activeIndex];
  const previewImage = activeChannel.profile.bannerUrl ?? activeChannel.profile.avatarUrl;
  const followerLabel = `${activeChannel.followerCount.toLocaleString()} follower${
    activeChannel.followerCount === 1 ? "" : "s"
  }`;

  return (
    <section
      className="featured-channel surface"
      role="region"
      aria-roledescription="carousel"
      aria-label="Featured channels carousel"
      aria-live="polite"
    >
      <div className="featured-channel__canvas">
        <div className="featured-channel__backdrop" aria-hidden="true">
          {previewImage && (
            <div
              className="featured-channel__backdrop-image"
              style={{ backgroundImage: `url(${previewImage})` }}
            />
          )}
          <div className="featured-channel__backdrop-layer" />
        </div>

        <article className="featured-channel__slide" aria-label={`Slide ${activeIndex + 1} of ${slides.length}`}>
          <div className="featured-channel__media">
            {previewImage ? (
              <img src={previewImage} alt={`${activeChannel.owner.displayName} channel artwork`} />
            ) : (
              <div className="featured-channel__media-fallback" aria-hidden="true" />
            )}
            <div className="overlay overlay--top overlay--scrim">
              {activeChannel.live ? <span className="badge badge--live">Live</span> : <span className="badge">Offline</span>}
              <span className="overlay__meta">{followerLabel}</span>
            </div>
          </div>
          <div className="featured-channel__content">
            <span className="featured-channel__eyebrow muted">Featured</span>
            <h2 className="featured-channel__title">{activeChannel.channel.title}</h2>
            <p className="featured-channel__subtitle muted">{activeChannel.owner.displayName}</p>
            <p className="muted">
              {activeChannel.profile.bio ?? "Get to know this creator’s story and tune into their latest broadcast."}
            </p>
            <div className="tag-list">
              {activeChannel.channel.category && <span className="tag">{activeChannel.channel.category}</span>}
              {activeChannel.channel.tags.slice(0, 3).map((tag) => (
                <span key={tag} className="tag">
                  #{tag}
                </span>
              ))}
            </div>
            <div className="featured-channel__actions">
              <Link className="primary-button" href={`/channels/${activeChannel.channel.id}`} aria-label="Watch featured channel">
                Watch now
              </Link>
              <Link className="secondary-button" href={`/channels/${activeChannel.channel.id}`} aria-label="View channel details">
                View channel
              </Link>
            </div>
          </div>
        </article>
      </div>

      <footer className="featured-channel__footer" aria-label="Featured channel controls">
        <div className="featured-channel__pagination" role="group" aria-label="Featured channel pagination">
          {slides.map((slide, index) => (
            <button
              key={slide.channel.id}
              type="button"
              className={`featured-channel__dot${index === activeIndex ? " featured-channel__dot--active" : ""}`}
              onClick={() => setActiveIndex(index)}
              aria-label={`Show featured channel ${slide.channel.title}`}
              aria-pressed={index === activeIndex}
            />
          ))}
        </div>
        <div className="featured-channel__controls" role="group" aria-label="Carousel navigation">
          <button
            type="button"
            className="secondary-button"
            onClick={() => setActiveIndex((activeIndex - 1 + slides.length) % slides.length)}
            aria-label="Previous featured channel"
            disabled={slides.length <= 1}
          >
            Previous
          </button>
          <button
            type="button"
            className="secondary-button"
            onClick={() => setActiveIndex((activeIndex + 1) % slides.length)}
            aria-label="Next featured channel"
            disabled={slides.length <= 1}
          >
            Next
          </button>
          <button
            type="button"
            className="secondary-button"
            onClick={() => setAutoPlayEnabled((prev) => !prev)}
            aria-label={`${autoPlayEnabled ? "Pause" : "Resume"} autoplay`}
          >
            {autoPlayEnabled ? "Pause" : "Play"}
          </button>
        </div>
      </footer>
    </section>
  );
}
