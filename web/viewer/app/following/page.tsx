"use client";

import Link from "next/link";
import { DirectoryGrid } from "../../components/DirectoryGrid";
import {
  FollowingEmptyPrompt,
  FollowingErrorBlock,
  FollowingLoadingBlock,
  FollowingUnauthenticatedPrompt,
} from "../../components/following/FollowingState";
import { useFollowingChannels } from "../../components/following/useFollowingChannels";
import { useAuth } from "../../hooks/useAuth";

export default function FollowingPage() {
  const { user, loading: authLoading } = useAuth();
  const { channels, status, reload, error } = useFollowingChannels({
    isAuthenticated: Boolean(user),
    authLoading,
  });
  const liveCategories = Array.from(
    new Set(channels.map((entry) => entry.channel.category).filter((category): category is string => Boolean(category))),
  );

  return (
    <div className="container container--wide following-page stack stack--xl">
      <header className="page-header surface">
        <div className="page-header__copy stack stack--sm">
          <div className="stack stack--2xs">
            <span className="page-eyebrow">Following</span>
            <h1>Stay close to your regular creators</h1>
          </div>
          <p className="muted">This is your fast lane back to the channels you already care about.</p>
        </div>
        <div className="page-header__actions">
          <Link href="/browse" className="secondary-button">
            Browse channels
          </Link>
        </div>
      </header>

      {status === "loading" ? (
        <div className="surface stack" role="status">
          <FollowingLoadingBlock />
        </div>
      ) : status === "unauthenticated" ? (
        <div className="surface stack stack--lg" role="status">
          <FollowingUnauthenticatedPrompt className="stack stack--sm" />
          <p className="muted">Browse the directory to find creators and follow them from their channel pages.</p>
          <Link href="/browse" className="secondary-button">
            Browse channels
          </Link>
        </div>
      ) : (
        <>
          {status === "error" ? (
            <div className="surface stack" role="alert">
              <FollowingErrorBlock
                onRetry={() => {
                  void reload();
                }}
              />
              {error ? <p className="muted">{error}</p> : null}
            </div>
          ) : null}

          {status === "empty" ? (
            <div className="surface stack stack--lg">
              <FollowingEmptyPrompt className="muted" />
              <p className="muted">
                Browse the directory to discover creators and follow them to see their streams here.
              </p>
              <Link href="/browse" className="primary-button">
                Browse channels
              </Link>
            </div>
          ) : status === "ready" ? (
            <>
              <section className="following-summary-grid">
                <article className="summary-card">
                  <span className="summary-card__label">Live now</span>
                  <strong className="summary-card__value">{channels.length}</strong>
                  <p className="muted">Creators you follow who are streaming right now.</p>
                </article>
                <article className="summary-card">
                  <span className="summary-card__label">Active categories</span>
                  <strong className="summary-card__value">{liveCategories.length}</strong>
                  <p className="muted">Distinct topics represented in your current following feed.</p>
                </article>
                <article className="summary-card">
                  <span className="summary-card__label">Next step</span>
                  <strong className="summary-card__value">Browse</strong>
                  <p className="muted">Use Browse when you want to discover beyond the channels already here.</p>
                </article>
              </section>

              <section className="surface stack">
                <div className="section-heading">
                  <div>
                    <h2>Live now</h2>
                    <p className="muted">Creators you follow who are currently streaming</p>
                  </div>
                  <span className="muted">{channels.length} live</span>
                </div>
                <DirectoryGrid channels={channels} />
              </section>
            </>
          ) : null}
        </>
      )}
    </div>
  );
}
