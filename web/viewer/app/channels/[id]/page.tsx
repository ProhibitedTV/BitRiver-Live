"use client";

import { useEffect, useRef, useState } from "react";
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
  const [activeTab, setActiveTab] = useState<"about" | "schedule" | "videos">("about");
  const { user } = useAuth();
  const previousUserIdRef = useRef<string | undefined>();
  const previousChannelIdRef = useRef<string | undefined>();

  useEffect(() => {
    let cancelled = false;
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
    void load(userChanged && !channelChanged ? false : shouldShowSpinner);
    const interval = setInterval(() => {
      void load(false);
    }, 30_000);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, [id, user?.id]);

  const handleFollowChange = (follow: FollowState) => {
    setData((prev) => (prev ? { ...prev, follow } : prev));
  };

  const handleSubscriptionChange = (subscription: SubscriptionState) => {
    setData((prev) => (prev ? { ...prev, subscription } : prev));
  };

  useEffect(() => {
    let cancelled = false;
    const loadVods = async () => {
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
      {error && <div className="surface">{error}</div>}
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
                  <VodGallery items={vods} error={vodError} />
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
