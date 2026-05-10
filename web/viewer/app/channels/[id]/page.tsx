"use client";

import { KeyboardEvent, useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { ChannelAboutPanel, ChannelHeader } from "../../../components/ChannelHero";
import { ChatPanel } from "../../../components/ChatPanel";
import { Player } from "../../../components/Player";
import { Button } from "../../../components/ui/Button";
import { Card, CardHeader } from "../../../components/ui/Card";
import { VodGallery } from "../../../components/VodGallery";
import { useAuth } from "../../../hooks/useAuth";
import type { ChannelPlaybackResponse, FollowState, SubscriptionState, VodItem } from "../../../lib/viewer-api";
import { fetchChannelPlayback, fetchChannelVods } from "../../../lib/viewer-api";

const CHANNEL_TABS = [
  { id: "about", label: "About" },
  { id: "schedule", label: "Schedule" },
  { id: "videos", label: "Videos" },
] as const;

type ChannelTabId = (typeof CHANNEL_TABS)[number]["id"];
const DEFAULT_CHANNEL_TAB: ChannelTabId = "about";

function parseChannelTab(value: string | null | undefined): ChannelTabId | undefined {
  if (!value) return undefined;
  const normalizedValue = value.toLowerCase();
  return CHANNEL_TABS.find((tab) => tab.id === normalizedValue)?.id;
}

function resolveTabFromUrl(searchParams: URLSearchParams, hash: string): ChannelTabId {
  return parseChannelTab(searchParams.get("tab")) ?? parseChannelTab(hash.replace(/^#/, "")) ?? DEFAULT_CHANNEL_TAB;
}

export default function ChannelPage({ params }: { params: { id: string } }) {
  const { id } = params;
  const [data, setData] = useState<ChannelPlaybackResponse | undefined>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | undefined>();
  const [vods, setVods] = useState<VodItem[]>([]);
  const [vodError, setVodError] = useState<string | undefined>();
  const [vodsLoading, setVodsLoading] = useState(false);
  const [activeTab, setActiveTab] = useState<ChannelTabId>(DEFAULT_CHANNEL_TAB);
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const tabSearchParam = searchParams.get("tab");
  const { user } = useAuth();
  const previousUserIdRef = useRef<string | undefined>();
  const previousChannelIdRef = useRef<string | undefined>();
  const refreshIntervalRef = useRef<NodeJS.Timeout | undefined>();
  const cancelledRef = useRef(false);
  const vodCancelledRef = useRef(false);
  const vodRequestedChannelIdRef = useRef<string | undefined>();
  const previousVodChannelIdRef = useRef<string | undefined>();
  const tabRefs = useRef<Array<HTMLButtonElement | null>>([]);

  const setTabFromCurrentUrl = useCallback(() => {
    if (typeof window === "undefined") return;
    const nextTab = resolveTabFromUrl(new URLSearchParams(window.location.search), window.location.hash);
    setActiveTab((currentTab) => (currentTab === nextTab ? currentTab : nextTab));
  }, []);

  const updateTabUrl = useCallback(
    (nextTab: ChannelTabId) => {
      const nextSearchParams = new URLSearchParams(searchParams.toString());
      nextSearchParams.set("tab", nextTab);
      const queryString = nextSearchParams.toString();
      const href = queryString ? `${pathname}?${queryString}` : pathname;
      router.push(href, { scroll: false });
    },
    [pathname, router, searchParams],
  );

  const clearRefreshInterval = useCallback(() => {
    if (!refreshIntervalRef.current) return;
    clearInterval(refreshIntervalRef.current);
    refreshIntervalRef.current = undefined;
  }, []);

  const loadPlayback = useCallback(
    async (showSpinner: boolean) => {
      try {
        if (showSpinner) setLoading(true);
        setError(undefined);
        const response = await fetchChannelPlayback(id);
        if (!cancelledRef.current) {
          setData(response);
        }
      } catch (err) {
        if (!cancelledRef.current) {
          setError(err instanceof Error ? err.message : "We couldn't load this channel.");
        }
      } finally {
        if (!cancelledRef.current && showSpinner) {
          setLoading(false);
        }
      }
    },
    [id],
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
      vodRequestedChannelIdRef.current = undefined;
      setTabFromCurrentUrl();
      setLoading(true);
    }
    previousUserIdRef.current = user?.id;
    previousChannelIdRef.current = id;
    void loadPlayback(userChanged && !channelChanged ? false : channelChanged || firstLoad);
    return () => {
      cancelledRef.current = true;
      clearRefreshInterval();
    };
  }, [clearRefreshInterval, id, loadPlayback, setTabFromCurrentUrl, user?.id]);

  useEffect(() => {
    setTabFromCurrentUrl();
  }, [setTabFromCurrentUrl, tabSearchParam]);

  useEffect(() => {
    if (typeof window === "undefined") return undefined;
    const handleLocationUpdate = () => setTabFromCurrentUrl();
    window.addEventListener("hashchange", handleLocationUpdate);
    window.addEventListener("popstate", handleLocationUpdate);
    return () => {
      window.removeEventListener("hashchange", handleLocationUpdate);
      window.removeEventListener("popstate", handleLocationUpdate);
    };
  }, [setTabFromCurrentUrl]);

  useEffect(() => {
    clearRefreshInterval();
    if (error) return undefined;
    refreshIntervalRef.current = setInterval(() => {
      void loadPlayback(false);
    }, 30000);
    return () => clearRefreshInterval();
  }, [clearRefreshInterval, error, loadPlayback]);

  const handleFollowChange = (follow: FollowState) => {
    setData((prev) => (prev ? { ...prev, follow } : prev));
  };

  const handleSubscriptionChange = (subscription: SubscriptionState) => {
    setData((prev) => (prev ? { ...prev, subscription } : prev));
  };

  const loadVods = useCallback(async () => {
    setVodsLoading(true);
    setVodError(undefined);
    try {
      const response = await fetchChannelVods(id);
      if (!vodCancelledRef.current) setVods(response.items ?? []);
    } catch (err) {
      if (!vodCancelledRef.current) {
        setVodError(err instanceof Error ? err.message : "We couldn't load past broadcasts.");
        setVods([]);
      }
    } finally {
      if (!vodCancelledRef.current) setVodsLoading(false);
    }
  }, [id]);

  const handleVodRetry = useCallback(() => {
    vodRequestedChannelIdRef.current = undefined;
    void loadVods();
  }, [loadVods]);

  useEffect(() => {
    vodCancelledRef.current = false;
    const channelChanged = previousVodChannelIdRef.current !== id;
    if (channelChanged) {
      previousVodChannelIdRef.current = id;
      vodRequestedChannelIdRef.current = undefined;
      return () => {
        vodCancelledRef.current = true;
      };
    }
    if (activeTab !== "videos") {
      return () => {
        vodCancelledRef.current = true;
      };
    }
    if (vodRequestedChannelIdRef.current === id) {
      return () => {
        vodCancelledRef.current = true;
      };
    }
    vodRequestedChannelIdRef.current = id;
    void loadVods();
    return () => {
      vodCancelledRef.current = true;
    };
  }, [activeTab, id, loadVods]);

  const activateTabAtIndex = useCallback(
    (index: number, focusTab: boolean) => {
      const normalizedIndex = (index + CHANNEL_TABS.length) % CHANNEL_TABS.length;
      const nextTabId = CHANNEL_TABS[normalizedIndex].id;
      setActiveTab(nextTabId);
      updateTabUrl(nextTabId);
      if (focusTab) tabRefs.current[normalizedIndex]?.focus();
    },
    [updateTabUrl],
  );

  const handleTabKeyDown = useCallback(
    (event: KeyboardEvent<HTMLButtonElement>, index: number) => {
      let nextIndex: number | undefined;
      switch (event.key) {
        case "ArrowRight":
        case "ArrowDown":
          nextIndex = index + 1;
          break;
        case "ArrowLeft":
        case "ArrowUp":
          nextIndex = index - 1;
          break;
        case "Home":
          nextIndex = 0;
          break;
        case "End":
          nextIndex = CHANNEL_TABS.length - 1;
          break;
        default:
          return;
      }
      event.preventDefault();
      activateTabAtIndex(nextIndex, true);
    },
    [activateTabAtIndex],
  );

  return (
    <div className="workspace-page workspace-page--narrow channel-page">
      {loading ? <Card className="workspace-card">Loading channel...</Card> : null}

      {error ? (
        <Card className="workspace-card" role="alert">
          <CardHeader className="workspace-card__header">
            <h2>We couldn&apos;t load this channel.</h2>
            <p className="muted">Something went wrong while fetching playback details. Please try again or return to the channel list.</p>
          </CardHeader>
          <div className="channel-page__actions">
            <Button onClick={handleRetry}>Try again</Button>
            <Link className="secondary-button" href="/browse">
              Back to channels
            </Link>
          </div>
          <p className="muted" aria-live="polite">
            If this keeps happening, refresh or return to Browse.
          </p>
          {process.env.NODE_ENV !== "production" ? (
            <details>
              <summary>Diagnostic details</summary>
              <pre className="muted" style={{ margin: 0, whiteSpace: "pre-wrap" }}>
                {error}
              </pre>
            </details>
          ) : null}
        </Card>
      ) : null}

      {data ? (
        <div className="channel-page__grid">
          <div className="channel-page__hero-grid">
            <div className="channel-player">
              <Player
                playback={data.playback}
                channelId={params.id}
                live={data.live}
                liveState={data.channel.liveState}
                loading={loading}
                onRetry={handleRetry}
                recoveryHref="/browse"
              />
            </div>
            <aside className="channel-page__chat">
              <div className="channel-page__chat-inner">
                <ChatPanel channelId={id} roomId={data.chat?.roomId} viewerCount={data.viewerCount} />
              </div>
            </aside>
          </div>

          <div className="channel-page__main stack">
            <ChannelHeader data={data} onFollowChange={handleFollowChange} onSubscriptionChange={handleSubscriptionChange} />

            <section className="channel-tabs">
              <div className="channel-tabs__list" role="tablist" aria-label="Stream info tabs">
                {CHANNEL_TABS.map((tab, index) => (
                  <button
                    key={tab.id}
                    ref={(element) => {
                      tabRefs.current[index] = element;
                    }}
                    id={`channel-tab-${tab.id}-trigger`}
                    role="tab"
                    type="button"
                    className="channel-tabs__trigger"
                    aria-selected={activeTab === tab.id}
                    tabIndex={activeTab === tab.id ? 0 : -1}
                    aria-controls={`channel-tab-${tab.id}`}
                    onClick={() => {
                      setActiveTab(tab.id);
                      updateTabUrl(tab.id);
                    }}
                    onKeyDown={(event) => handleTabKeyDown(event, index)}
                  >
                    {tab.label}
                  </button>
                ))}
              </div>
              <div className="channel-tabs__panels">
                <div id="channel-tab-about" role="tabpanel" aria-labelledby="channel-tab-about-trigger" hidden={activeTab !== "about"} className="channel-tabs__panel">
                  <ChannelAboutPanel data={data} />
                </div>
                <div id="channel-tab-schedule" role="tabpanel" aria-labelledby="channel-tab-schedule-trigger" hidden={activeTab !== "schedule"} className="channel-tabs__panel">
                  <section className="surface stack">
                    <h3>Schedule</h3>
                    <p className="muted">The broadcaster hasn&apos;t shared an upcoming schedule yet.</p>
                  </section>
                </div>
                <div id="channel-tab-videos" role="tabpanel" aria-labelledby="channel-tab-videos-trigger" hidden={activeTab !== "videos"} className="channel-tabs__panel">
                  <VodGallery items={vods} error={vodError} loading={vodsLoading} onRetry={handleVodRetry} />
                </div>
              </div>
            </section>

            {user?.id === data.channel.ownerId || user?.roles.includes("creator") ? (
              <section className="channel-owner-card">
                <span className="page-eyebrow">Creator tools</span>
                <h3>Manage uploads</h3>
                <p className="muted">Use your creator dashboard to register VODs and monitor processing once streams finish.</p>
                <div className="channel-page__actions">
                  <Link href={`/creator/uploads/${data.channel.id}`} className="secondary-button">
                    Open creator dashboard
                  </Link>
                </div>
              </section>
            ) : null}
          </div>
        </div>
      ) : null}
    </div>
  );
}
