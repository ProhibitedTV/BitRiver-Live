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

  return (
    <div className="container stack">
      <header className="stack">
        <h1>Following</h1>
        <p className="muted">Catch live broadcasts from creators you follow.</p>
      </header>

      {status === "loading" ? (
        <div className="surface" role="status">
          <FollowingLoadingBlock />
        </div>
      ) : status === "unauthenticated" ? (
        <div className="surface stack" role="status">
          <FollowingUnauthenticatedPrompt className="stack" />
          <p className="muted">Browse the directory to find creators and follow them from their channel pages.</p>
          <Link href="/browse" className="secondary-button" prefetch>
            Browse channels
          </Link>
        </div>
      ) : (
        <>
          {status === "error" ? (
            <div className="surface" role="alert">
              <FollowingErrorBlock
                onRetry={() => {
                  void reload();
                }}
              />
              {error ? <p className="muted">{error}</p> : null}
            </div>
          ) : null}

          {status === "empty" ? (
            <div className="surface stack">
              <FollowingEmptyPrompt className="muted" />
              <p className="muted">
                Browse the directory to discover creators and follow them to see their streams here.
              </p>
              <Link href="/browse" className="primary-button" prefetch>
                Browse channels
              </Link>
            </div>
          ) : status === "ready" ? (
            <section className="stack">
              <div className="section-heading">
                <div>
                  <h2>Live now</h2>
                  <p className="muted">Creators you follow who are currently streaming</p>
                </div>
                <span className="muted">{channels.length} live</span>
              </div>
              <DirectoryGrid channels={channels} />
            </section>
          ) : null}
        </>
      )}
    </div>
  );
}
