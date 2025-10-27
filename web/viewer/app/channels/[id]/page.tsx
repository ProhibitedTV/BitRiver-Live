"use client";

import { useEffect, useState } from "react";
import { ChannelHero } from "../../../components/ChannelHero";
import { Player } from "../../../components/Player";
import type { ChannelPlaybackResponse, FollowState } from "../../../lib/viewer-api";
import { fetchChannelPlayback } from "../../../lib/viewer-api";

export default function ChannelPage({ params }: { params: { id: string } }) {
  const { id } = params;
  const [data, setData] = useState<ChannelPlaybackResponse | undefined>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | undefined>();

  useEffect(() => {
    let cancelled = false;
    const load = async (showSpinner: boolean) => {
      try {
        if (showSpinner) {
          setLoading(true);
        }
        setError(undefined);
        const response = await fetchChannelPlayback(id);
        if (!cancelled) {
          setData(response);
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Unable to load channel");
        }
      } finally {
        if (!cancelled && showSpinner) {
          setLoading(false);
        }
      }
    };
    void load(true);
    const interval = setInterval(() => {
      void load(false);
    }, 30_000);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, [id]);

  const handleFollowChange = (follow: FollowState) => {
    setData((prev) => (prev ? { ...prev, follow } : prev));
  };

  return (
    <div className="container stack">
      {loading && <div className="surface">Loading channel…</div>}
      {error && <div className="surface">{error}</div>}
      {data && (
        <>
          <ChannelHero data={data} onFollowChange={handleFollowChange} />
          <Player playback={data.playback} />
          {data.playback?.renditions && data.playback.renditions.length > 0 && (
            <div className="surface stack">
              <h3>Available renditions</h3>
              <ul>
                {data.playback.renditions.map((rendition) => (
                  <li key={rendition.name}>
                    <strong>{rendition.name}</strong> &middot; {rendition.manifestUrl}
                    {rendition.bitrate && ` · ${Math.round(rendition.bitrate / 1000)} kbps`}
                  </li>
                ))}
              </ul>
            </div>
          )}
        </>
      )}
    </div>
  );
}
