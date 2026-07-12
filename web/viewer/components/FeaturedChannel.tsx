"use client";

import { useEffect, useMemo, useState } from "react";
import Image from "next/image";
import Link from "next/link";
import { ChannelStatusBadge } from "./channel/ChannelStatusBadge";
import { formatFollowerLabel, getChannelPreviewImage } from "../lib/channel-presenters";
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
  const [prefersReducedMotion, setPrefersReducedMotion] = useState(false);
  const [autoPlayOverride, setAutoPlayOverride] = useState<boolean | null>(null);

  useEffect(() => {
    if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
      return undefined;
    }

    const mediaQuery = window.matchMedia("(prefers-reduced-motion: reduce)");
    setPrefersReducedMotion(mediaQuery.matches);

    const handleReducedMotionChange = (event: MediaQueryListEvent) => {
      setPrefersReducedMotion(event.matches);
    };

    if (typeof mediaQuery.addEventListener === "function") {
      mediaQuery.addEventListener("change", handleReducedMotionChange);
      return () => mediaQuery.removeEventListener("change", handleReducedMotionChange);
    }

    mediaQuery.addListener(handleReducedMotionChange);
    return () => mediaQuery.removeListener(handleReducedMotionChange);
  }, []);

  useEffect(() => {
    setActiveIndex(0);
  }, [slides.length]);

  const autoPlayEnabled = slides.length > 1 && (autoPlayOverride ?? (autoPlay && !prefersReducedMotion));

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
      <div className="featured-channel" aria-busy="true" aria-live="polite">
        <div className="state-panel state-panel--loading">
          <strong>Loading featured stream</strong>
          <p className="muted">Selecting a standout broadcast for the top of the page.</p>
        </div>
      </div>
    );
  }

  if (!slides.length) {
    return (
      <div className="featured-channel" role="region" aria-label="Featured channels">
        <div className="state-panel">
          <strong>Awaiting first broadcast</strong>
          <p className="muted">
            The public relay is ready. Its first live channel will be promoted here automatically.
          </p>
          <div className="browse-actions">
            <Link href="/browse" className="secondary-button">
              Browse full directory
            </Link>
          </div>
        </div>
      </div>
    );
  }

  const activeChannel = slides[activeIndex];
  const previewImage = getChannelPreviewImage(activeChannel);
  const canNavigate = slides.length > 1;
  const autoPlayToggleLabel = autoPlayEnabled ? "Pause autoplay" : "Resume autoplay";

  const showPrevious = () => {
    if (!canNavigate) {
      return;
    }
    setActiveIndex((current) => (current - 1 + slides.length) % slides.length);
  };

  const showNext = () => {
    if (!canNavigate) {
      return;
    }
    setActiveIndex((current) => (current + 1) % slides.length);
  };

  const toggleAutoPlay = () => {
    setAutoPlayOverride((current) => !(current ?? (autoPlay && !prefersReducedMotion)));
  };

  return (
    <section
      className="featured-channel"
      role="region"
      aria-roledescription="carousel"
      aria-label="Featured channels carousel"
      aria-live="polite"
    >
      <div className="featured-channel__canvas">
        <div className="featured-channel__backdrop" aria-hidden="true">
          {previewImage && <div className="featured-channel__backdrop-image" style={{ backgroundImage: `url(${previewImage})` }} />}
          <div className="featured-channel__backdrop-layer" />
        </div>

        <article className="featured-channel__slide" aria-label={`Slide ${activeIndex + 1} of ${slides.length}`}>
          <div className="featured-channel__media">
            {previewImage ? (
              <Image
                src={previewImage}
                alt={`${activeChannel.owner.displayName} channel artwork`}
                fill
                sizes="(min-width: 1280px) 40vw, 100vw"
                className="featured-channel__media-image"
                priority
              />
            ) : (
              <div className="featured-channel__media-fallback" aria-hidden="true" />
            )}
            <div className="overlay overlay--top overlay--scrim">
              <ChannelStatusBadge live={activeChannel.live} offlineClassName="" />
              <span className="overlay__meta">{formatFollowerLabel(activeChannel.followerCount)}</span>
            </div>
          </div>
            <div className="featured-channel__content">
            <span className="featured-channel__eyebrow muted">Featured live</span>
            <h2 className="featured-channel__title">{activeChannel.channel.title}</h2>
            <p className="featured-channel__subtitle muted">{activeChannel.owner.displayName}</p>
            <p className="muted">
              {activeChannel.profile.bio ?? "Open the channel to watch live and follow updates from this creator."}
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
              <Link
                className="primary-button"
                href={`/channels/${activeChannel.channel.id}`}
                aria-label="View featured channel"
              >
                Watch channel
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
          <button type="button" className="secondary-button" onClick={showPrevious} disabled={!canNavigate} aria-label="Previous featured channel">
            Back
          </button>
          <button type="button" className="secondary-button" onClick={showNext} disabled={!canNavigate} aria-label="Next featured channel">
            Next
          </button>
          <button type="button" className="secondary-button" onClick={toggleAutoPlay} aria-label={autoPlayToggleLabel}>
            {autoPlayEnabled ? "Pause" : "Play"}
          </button>
        </div>
      </footer>
    </section>
  );
}
