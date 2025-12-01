"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { ChannelAboutPanel, ChannelHeader } from "../../../components/ChannelHero";
import { ChatPanel } from "../../../components/ChatPanel";
import { Player } from "../../../components/Player";
import { VodGallery } from "../../../components/VodGallery";
import { useAuth } from "../../../hooks/useAuth";
import type {
  ChannelPlaybackResponse,
  FollowState,
  SubscriptionState,
  VodItem
} from "../../../lib/viewer-api";
import { fetchChannelPlayback, fetchChannelVods } from "../../../lib/viewer-api";

export default function ChannelPage({ params }: { params: { id: string } }) {
  const { id } = params;
  const [data, setData] = useState<ChannelPlaybackResponse | undefined>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | undefined>();
  const [vods, setVods] = useState<VodItem[]>([]);
  const [vodError, setVodError] = useState<string | undefined>();
  const [vodsLoading, setVodsLoading] = useState(false);
  const [activeTab, setActiveTab] = useState<"about" | "schedule" | "videos">("about");
  const { user } = useAuth();
  const previousUserIdRef = useRef<string | undefined>();
  const previousChannelIdRef = useRef<string | undefined>();
  const refreshIntervalRef = useRef<NodeJS.Timeout | undefined>();
  const cancelledRef = useRef(false);

  const clearRefreshInterval = useCallback(() => {
    if (refreshIntervalRef.current) {
      clearInterval(refreshIntervalRef.current);
      refreshIntervalRef.current = undefined;
    }
  }, []);

  const loadPlayback = useCallback(
    async (showSpinner: boolean) => {
      try {
        if (showSpinner) {
          setLoading(true);
        }
        setError(undefined);
        const response = await fetchChannelPlayback(id);
        if (!cancelledRef.current) {
          setData(response);
        }
      } catch (err) {
        if (!cancelledRef.current) {
          setError(err instanceof Error ? err.message : "Unable to load channel");
        }
      } finally {
        if (!cancelledRef.current && showSpinner) {
          setLoading(false);
        }
      }
    },
    [id]
  );

  const handleRetry = useCallback(() => {
    void loadPlayback(true);
  }, [loadPlayback]);

  useEffect(() => {
    cancelledRef.current = false;
    const previousUserId = previousUserIdRef.current;
    const previousChannelId = previousChannelIdRef.current;
    const channelChanged = previousChannelId !== id;
    const firstLoad = previousChannelId === undefined;
    const userChanged = previousUserId !== user?.id;
    if (channelChanged) {
      setData(undefined);
      setVods([]);
      setVodError(undefined);
      setActiveTab("about");
      setLoading(true);
    }
    previousUserIdRef.current = user?.id;
    previousChannelIdRef.current = id;
    const shouldShowSpinner = channelChanged || firstLoad;
    void loadPlayback(userChanged && !channelChanged ? false : shouldShowSpinner);
    return () => {
      cancelledRef.current = true;
      clearRefreshInterval();
    };
  }, [clearRefreshInterval, id, loadPlayback, user?.id]);

  useEffect(() => {
    clearRefreshInterval();
    if (error) {
      return undefined;
    }
    refreshIntervalRef.current = setInterval(() => {
      void loadPlayback(false);
    }, 30_000);
    return () => {
      clearRefreshInterval();
    };
  }, [clearRefreshInterval, error, loadPlayback]);

  const handleFollowChange = (follow: FollowState) => {
    setData((prev) => (prev ? { ...prev, follow } : prev));
  };

  const handleSubscriptionChange = (subscription: SubscriptionState) => {
    setData((prev) => (prev ? { ...prev, subscription } : prev));
  };

  useEffect(() => {
    let cancelled = false;
    const loadVods = async () => {
      setVodsLoading(true);
      try {
        const response = await fetchChannelVods(id);
        if (!cancelled) {
          setVodError(undefined);
          setVods(response.items ?? []);
        }
      } catch (err) {
        if (!cancelled) {
          setVodError(err instanceof Error ? err.message : "Unable to load replays");
          setVods([]);
        }
      } finally {
        if (!cancelled) {
          setVodsLoading(false);
        }
      }
    };
    void loadVods();
    return () => {
      cancelled = true;
    };
  }, [id]);

  const tabs = [
    { id: "about", label: "About" },
    { id: "schedule", label: "Schedule" },
    { id: "videos", label: "Videos" }
  ] as const;

  return (
    <div className="container channel-page">
      {loading && <div className="surface">Loading channel…</div>}
      {error && (
        <div className="surface stack" role="alert">
          <div className="stack">
            <h2>We couldn&apos;t load this channel.</h2>
            <p className="muted">
              Something went wrong while fetching playback details. Please try again or return to the channel list.
            </p>
          </div>
          <div className="cluster" style={{ justifyContent: "flex-start", gap: "var(--space-3)" }}>
            <button className="button" onClick={handleRetry} type="button">
              Try again
            </button>
            <Link className="secondary-button" href="/browse">
              Back to channels
            </Link>
          </div>
          <p className="muted" aria-live="polite">
            Error details: {error}
          </p>
        </div>
      )}
      {data && (
        <div className="channel-page__grid">
          <div className="channel-page__hero-grid">
            <div className="channel-player">
              <Player playback={data.playback} />
            </div>
            <aside className="channel-page__chat">
              <div className="channel-page__chat-inner">
                <ChatPanel channelId={id} roomId={data.chat?.roomId} viewerCount={data.viewerCount} />
              </div>
            </aside>
          </div>
          <div className="channel-page__main stack">
            <ChannelHeader
              data={data}
              onFollowChange={handleFollowChange}
              onSubscriptionChange={handleSubscriptionChange}
            />
            <section className="channel-tabs">
              <div className="channel-tabs__list" role="tablist" aria-label="Stream info tabs">
                {tabs.map((tab) => (
                  <button
                    key={tab.id}
                    id={`channel-tab-${tab.id}-trigger`}
                    role="tab"
                    type="button"
                    className="channel-tabs__trigger"
                    aria-selected={activeTab === tab.id}
                    aria-controls={`channel-tab-${tab.id}`}
                    onClick={() => setActiveTab(tab.id)}
                  >
                    {tab.label}
                  </button>
                ))}
              </div>
              <div className="channel-tabs__panels">
                <div
                  id="channel-tab-about"
                  role="tabpanel"
                  aria-labelledby="channel-tab-about-trigger"
                  hidden={activeTab !== "about"}
                  className="channel-tabs__panel"
                >
                  <ChannelAboutPanel data={data} />
                </div>
                <div
                  id="channel-tab-schedule"
                  role="tabpanel"
                  aria-labelledby="channel-tab-schedule-trigger"
                  hidden={activeTab !== "schedule"}
                  className="channel-tabs__panel"
                >
                  <section className="surface stack">
                    <h3>Schedule</h3>
                    <p className="muted">The broadcaster hasn&apos;t shared an upcoming schedule yet.</p>
                  </section>
                </div>
                <div
                  id="channel-tab-videos"
                  role="tabpanel"
                  aria-labelledby="channel-tab-videos-trigger"
                  hidden={activeTab !== "videos"}
                  className="channel-tabs__panel"
                >
                  <VodGallery items={vods} error={vodError} loading={vodsLoading} />
                </div>
              </div>
            </section>
            {(user?.id === data.channel.ownerId || user?.roles.includes("creator")) && (
              <section className="surface stack">
                <header className="stack">
                  <h3>Manage uploads</h3>
                  <p className="muted">
                    Use your creator dashboard to register VODs and monitor processing once streams finish.
                  </p>
                </header>
                <Link
                  href={`/creator/uploads/${data.channel.id}`}
                  className="secondary-button"
                  style={{ alignSelf: "flex-start" }}
                >
                  Open creator dashboard
                </Link>
              </section>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
