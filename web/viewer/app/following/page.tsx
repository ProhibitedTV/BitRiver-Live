"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { DirectoryGrid } from "../../components/DirectoryGrid";
import { useAuth } from "../../hooks/useAuth";
import type { DirectoryChannel } from "../../lib/viewer-api";
import { fetchFollowingChannels } from "../../lib/viewer-api";

export default function FollowingPage() {
  const { user, loading: authLoading } = useAuth();
  const [channels, setChannels] = useState<DirectoryChannel[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | undefined>();

  useEffect(() => {
    let cancelled = false;

    if (authLoading) {
      return () => {
        cancelled = true;
      };
    }

    if (!user) {
      setChannels([]);
      setError(undefined);
      setLoading(false);
      return () => {
        cancelled = true;
      };
    }

    const load = async () => {
      try {
        setLoading(true);
        setError(undefined);
        const response = await fetchFollowingChannels();
        if (!cancelled) {
          setChannels(response.channels);
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Unable to load followed channels");
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    };

    void load();

    return () => {
      cancelled = true;
    };
  }, [authLoading, user]);

  return (
    <div className="container stack">
      <header className="stack">
        <h1>Following</h1>
        <p className="muted">Catch live broadcasts from creators you follow.</p>
      </header>

      {authLoading ? (
        <div className="surface" role="status">
          Checking your session…
        </div>
      ) : !user ? (
        <div className="surface stack" role="status">
          <p className="muted">Sign in to see channels you follow.</p>
          <p className="muted">Browse the directory to find creators and follow them from their channel pages.</p>
          <Link href="/browse" className="primary-button" prefetch>
            Browse channels
          </Link>
        </div>
      ) : (
        <>
          {error ? (
            <div className="surface" role="alert">
              We couldn&apos;t load your followed channels. {error}
            </div>
          ) : null}

          {loading ? (
            <div className="surface">Checking who is live…</div>
          ) : channels.length === 0 ? (
            <div className="surface stack">
              <p className="muted">You&rsquo;re not following any channels yet.</p>
              <p className="muted">
                Browse the directory to discover creators and follow them to see their streams here.
              </p>
              <Link href="/browse" className="primary-button" prefetch>
                Browse channels
              </Link>
            </div>
          ) : (
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
          )}
        </>
      )}
    </div>
  );
}
