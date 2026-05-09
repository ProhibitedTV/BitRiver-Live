"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import {
  fetchChannelVods,
  fetchLiveNowChannels,
  fetchRecommendedChannels,
  fetchTrendingChannels,
  type DirectoryChannel,
  type VodItem,
} from "../../lib/viewer-api";

type ReplayShelf = {
  channel: DirectoryChannel;
  items: VodItem[];
};

type ReplayCard = {
  channel: DirectoryChannel;
  item: VodItem;
};

function formatDuration(durationSeconds: number) {
  const totalMinutes = Math.max(1, Math.round(durationSeconds / 60));
  if (totalMinutes < 60) {
    return `${totalMinutes} min`;
  }

  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;
  return minutes === 0 ? `${hours} hr` : `${hours} hr ${minutes} min`;
}

function uniqueChannels(channelLists: DirectoryChannel[][]) {
  const seen = new Set<string>();
  const combined: DirectoryChannel[] = [];

  channelLists.forEach((channels) => {
    channels.forEach((channel) => {
      if (seen.has(channel.channel.id)) {
        return;
      }
      seen.add(channel.channel.id);
      combined.push(channel);
    });
  });

  return combined;
}

export default function VideosPage() {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | undefined>();
  const [shelves, setShelves] = useState<ReplayShelf[]>([]);

  useEffect(() => {
    let cancelled = false;

    const loadVideos = async () => {
      setLoading(true);
      setError(undefined);

      try {
        const [recommendedResult, trendingResult, liveNowResult] = await Promise.allSettled([
          fetchRecommendedChannels(),
          fetchTrendingChannels(),
          fetchLiveNowChannels(),
        ]);

        const channelCandidates = uniqueChannels([
          recommendedResult.status === "fulfilled" ? recommendedResult.value.channels : [],
          trendingResult.status === "fulfilled" ? trendingResult.value.channels : [],
          liveNowResult.status === "fulfilled" ? liveNowResult.value.channels : [],
        ]).slice(0, 8);

        const replayResults = await Promise.all(
          channelCandidates.map(async (channel) => {
            try {
              const collection = await fetchChannelVods(channel.channel.id);
              return { channel, items: collection.items ?? [] };
            } catch {
              return { channel, items: [] };
            }
          }),
        );

        if (!cancelled) {
          setShelves(replayResults.filter((entry) => entry.items.length > 0));
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Unable to load replay shelves");
          setShelves([]);
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    };

    void loadVideos();

    return () => {
      cancelled = true;
    };
  }, []);

  const replayCards = useMemo<ReplayCard[]>(() => {
    return shelves
      .flatMap((shelf) => shelf.items.map((item) => ({ channel: shelf.channel, item })))
      .sort((left, right) => new Date(right.item.publishedAt).getTime() - new Date(left.item.publishedAt).getTime())
      .slice(0, 8);
  }, [shelves]);

  return (
    <div className="container container--wide videos-page stack stack--xl">
      <header className="page-header surface surface--glow">
        <div className="page-header__copy stack stack--sm">
          <div className="stack stack--2xs">
            <span className="page-eyebrow">Videos</span>
            <h1>Catch recent broadcasts without waiting for the next stream</h1>
          </div>
          <p className="muted">
            Replays stay close to the live product so viewers can move between current streams, recent sessions, and creator pages without losing context.
          </p>
        </div>
        <div className="page-header__actions">
          <Link href="/browse" className="secondary-button">
            Browse live channels
          </Link>
          <Link href="/creator/getting-started" className="primary-button">
            Go live yourself
          </Link>
        </div>
      </header>

      <section className="surface stack stack--md">
        <div className="section-heading">
          <div>
            <h2>Recent replays</h2>
            <p className="muted">Open a replay hub fast, then jump into the creator page that owns the VOD.</p>
          </div>
          {!loading && !error && <span className="muted">{replayCards.length} replay picks</span>}
        </div>

        {loading ? (
          <div className="state-panel state-panel--loading" aria-busy="true">
            <strong>Loading replays</strong>
            <p className="muted">Checking recent creator uploads and past broadcasts now.</p>
          </div>
        ) : error ? (
          <div className="state-panel state-panel--error" role="alert">
            <strong>Replay shelves unavailable</strong>
            <p className="muted">{error}</p>
          </div>
        ) : replayCards.length === 0 ? (
          <div className="state-panel">
            <strong>No public replays yet</strong>
            <p className="muted">Creators can publish VODs after a stream ends. Live channels are still ready to watch now.</p>
            <div className="browse-actions">
              <Link href="/browse" className="primary-button">
                Watch live channels
              </Link>
              <Link href="/creator/getting-started" className="secondary-button">
                Open creator setup
              </Link>
            </div>
          </div>
        ) : (
          <div className="video-grid">
            {replayCards.map(({ channel, item }) => (
              <article key={`${channel.channel.id}-${item.id}`} className="video-card surface">
                <div className="video-card__header">
                  <span className="page-eyebrow">Replay</span>
                  <span className="muted">{formatDuration(item.durationSeconds)}</span>
                </div>
                <div className="stack stack--2xs">
                  <h3>{item.title}</h3>
                  <p className="muted">
                    {channel.owner.displayName} in {channel.channel.category ?? "Streaming"}
                  </p>
                </div>
                <p className="muted">Published {new Date(item.publishedAt).toLocaleDateString()}</p>
                <div className="video-card__actions">
                  <Link href={`/channels/${channel.channel.id}?tab=videos`} className="primary-button">
                    Open replays
                  </Link>
                  <Link href={`/channels/${channel.channel.id}`} className="secondary-button">
                    Open channel
                  </Link>
                </div>
              </article>
            ))}
          </div>
        )}
      </section>

      {!loading && !error && shelves.length > 0 && (
        <section className="surface stack stack--md">
          <div className="section-heading">
            <div>
              <h2>Channels with recent replays</h2>
              <p className="muted">These creators already have public VOD history available from their channel pages.</p>
            </div>
            <span className="muted">{shelves.length} channels</span>
          </div>

          <div className="video-channel-grid">
            {shelves.map(({ channel, items }) => (
              <article key={channel.channel.id} className="video-channel-card surface">
                <div className="stack stack--2xs">
                  <span className="page-eyebrow">{items.length} replay{items.length === 1 ? "" : "s"}</span>
                  <h3>{channel.channel.title}</h3>
                  <p className="muted">{channel.owner.displayName}</p>
                </div>
                <p className="muted">
                  Latest replay: {items[0]?.title ?? "Unavailable"}
                </p>
                <div className="video-channel-card__actions">
                  <Link href={`/channels/${channel.channel.id}?tab=videos`} className="secondary-button">
                    View channel videos
                  </Link>
                </div>
              </article>
            ))}
          </div>
        </section>
      )}
    </div>
  );
}
