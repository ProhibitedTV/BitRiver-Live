"use client";

import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { Player } from "../../../../components/Player";
import { Button, buttonClassName } from "../../../../components/ui/Button";
import { Card, CardBody, CardHeader } from "../../../../components/ui/Card";
import { InlineAlert } from "../../../../components/ui/InlineAlert";
import { useAuth } from "../../../../hooks/useAuth";
import { useCreatorChannel } from "../../../../hooks/useCreatorChannel";
import { buildViewerPath, buildViewerUrl } from "../../../../lib/viewer-links";
import { deriveCreatorGoLiveStatus } from "./stream-status";
import {
  ManagedChannel,
  StreamSession,
  fetchChannelSessions,
  fetchManagedChannels,
  updateChannel,
} from "../../../../lib/viewer-api";

const MASKED_STREAM_KEY = "••••••••";

function formatCategory(category?: string) {
  if (!category) {
    return "Uncategorized";
  }
  return category;
}

function describeEndpoint(endpoint: string, index: number) {
  if (index === 0) {
    return "Primary ingest";
  }
  if (index === 1) {
    return "Backup ingest";
  }
  return `Ingest ${index + 1}`;
}

function getPreferredIngestEndpoint(endpoints: string[]) {
  const rtmpEndpoint = endpoints.find((endpoint) => endpoint.toLowerCase().startsWith("rtmp://"));
  return rtmpEndpoint ?? endpoints[0];
}

function formatTimestamp(timestamp?: string) {
  if (!timestamp) {
    return "Checking now...";
  }
  return new Date(timestamp).toLocaleString();
}

function buildObsSettingsBlock(preferredIngestEndpoint?: string, streamKey?: string, streamKeyVisible = false) {
  const serverLine = preferredIngestEndpoint ? `Server: ${preferredIngestEndpoint}` : "Server: [not available yet]";
  const streamKeyLine = streamKey
    ? `Stream Key: ${streamKeyVisible ? streamKey : "[hidden - reveal to copy]"}`
    : "Stream Key: [owner access required]";

  return `Service: Custom\n${serverLine}\n${streamKeyLine}`;
}

export default function CreatorLivePage() {
  const { playback, loading, error, channelId, reload } = useCreatorChannel();
  const { user, loading: authLoading } = useAuth();
  const [sessionError, setSessionError] = useState<string | undefined>();
  const [sessions, setSessions] = useState<StreamSession[]>([]);
  const [managedChannel, setManagedChannel] = useState<ManagedChannel | undefined>();
  const [managedChannels, setManagedChannels] = useState<ManagedChannel[]>([]);
  const [managedLoading, setManagedLoading] = useState(true);
  const [managedError, setManagedError] = useState<string | undefined>();
  const [titleDraft, setTitleDraft] = useState("");
  const [savingTitle, setSavingTitle] = useState(false);
  const [titleError, setTitleError] = useState<string | undefined>();
  const [titleSaved, setTitleSaved] = useState(false);
  const [streamKeyVisible, setStreamKeyVisible] = useState(false);
  const [streamKeyCopyMessage, setStreamKeyCopyMessage] = useState<string | undefined>();
  const [ingestCopyMessage, setIngestCopyMessage] = useState<string | undefined>();
  const [copiedIngestEndpoint, setCopiedIngestEndpoint] = useState<string | undefined>();
  const [obsSettingsCopyMessage, setObsSettingsCopyMessage] = useState<string | undefined>();
  const [viewerLinkCopyMessage, setViewerLinkCopyMessage] = useState<string | undefined>();
  const [testStreamUpdatedAt, setTestStreamUpdatedAt] = useState<string | undefined>();
  const router = useRouter();

  const loadSessions = useCallback(async () => {
    setSessionError(undefined);
    try {
      const response = await fetchChannelSessions(channelId);
      setSessions(response ?? []);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unable to load ingest details";
      setSessionError(message);
    }
  }, [channelId]);

  const loadManagedChannel = useCallback(async () => {
    setManagedLoading(true);
    setManagedError(undefined);
    try {
      const channels = await fetchManagedChannels();
      setManagedChannels(channels);
      const match = channels.find((channel) => channel.id === channelId);
      setManagedChannel(match);
      if (!match) {
        setManagedError(channels.length > 0 ? "Channel access unavailable" : "No managed channels available");
      }
    } catch (err) {
      setManagedChannels([]);
      setManagedChannel(undefined);
      const message = err instanceof Error ? err.message : "Unable to load channel settings";
      setManagedError(message);
    } finally {
      setManagedLoading(false);
    }
  }, [channelId]);

  const refreshNow = useCallback(async () => {
    await Promise.allSettled([reload(true), loadSessions(), loadManagedChannel()]);
    setTestStreamUpdatedAt(new Date().toISOString());
  }, [loadManagedChannel, loadSessions, reload]);

  useEffect(() => {
    void refreshNow();
  }, [refreshNow]);

  useEffect(() => {
    setTitleDraft(playback?.channel.title ?? "");
    setTitleSaved(false);
    setTitleError(undefined);
  }, [playback?.channel.title]);

  useEffect(() => {
    setStreamKeyVisible(false);
    setStreamKeyCopyMessage(undefined);
    setIngestCopyMessage(undefined);
    setObsSettingsCopyMessage(undefined);
    setViewerLinkCopyMessage(undefined);
  }, [channelId, managedChannel?.id]);

  useEffect(() => {
    const pollId = window.setInterval(() => {
      void refreshNow();
    }, 4000);

    return () => {
      window.clearInterval(pollId);
    };
  }, [refreshNow]);

  useEffect(() => {
    if (!copiedIngestEndpoint) {
      return;
    }

    const timeoutId = window.setTimeout(() => {
      setCopiedIngestEndpoint(undefined);
    }, 1500);

    return () => {
      window.clearTimeout(timeoutId);
    };
  }, [copiedIngestEndpoint]);

  const handleChannelChange = (event: FormEvent<HTMLSelectElement>) => {
    const nextChannelId = event.currentTarget.value;
    if (nextChannelId && nextChannelId !== channelId) {
      void router.push(`/creator/live/${nextChannelId}`);
    }
  };

  const latestSession = useMemo(() => {
    if (sessions.length === 0) {
      return undefined;
    }
    const sorted = [...sessions].sort(
      (a, b) => new Date(b.startedAt).getTime() - new Date(a.startedAt).getTime(),
    );
    return sorted.find((session) => !session.endedAt) ?? sorted[0];
  }, [sessions]);

  const isChannelOwner = useMemo(
    () => Boolean(managedChannel && user && managedChannel.ownerId === user.id),
    [managedChannel, user],
  );

  const ingestEndpoints = useMemo(() => managedChannel?.ingestEndpoints ?? [], [managedChannel?.ingestEndpoints]);
  const preferredIngestEndpoint = useMemo(() => getPreferredIngestEndpoint(ingestEndpoints), [ingestEndpoints]);
  const obsSettingsBlock = useMemo(
    () =>
      buildObsSettingsBlock(
        preferredIngestEndpoint,
        isChannelOwner ? managedChannel?.streamKey : undefined,
        streamKeyVisible,
      ),
    [isChannelOwner, managedChannel?.streamKey, preferredIngestEndpoint, streamKeyVisible],
  );
  const testStreamStatus = useMemo(
    () =>
      deriveCreatorGoLiveStatus(
        Boolean(playback?.live),
        playback?.channel.liveState,
        playback?.channel.currentSessionId,
        latestSession,
      ),
    [latestSession, playback?.channel.currentSessionId, playback?.channel.liveState, playback?.live],
  );
  const previewReady = Boolean(playback?.playback?.playbackUrl);
  const previewPending = Boolean(!previewReady && (testStreamStatus.key === "reconnecting" || playback?.live));
  const currentChannelTitle = managedChannel?.title ?? playback?.channel.title ?? "";
  const currentChannelCategory = formatCategory(managedChannel?.category ?? playback?.channel.category);
  const viewerPageHref = useMemo(() => {
    if (typeof window === "undefined") {
      return buildViewerPath(`/channels/${channelId}`);
    }
    return buildViewerUrl(`/channels/${channelId}`, window.location.origin);
  }, [channelId]);

  const previewMessage = useMemo(() => {
    if (previewReady && testStreamStatus.key === "live") {
      return "Your live preview is ready. Confirm video and audio here before you share the viewer link.";
    }

    if (previewPending) {
      return "BitRiver is receiving your stream, but the preview player is still warming up. Keep OBS running and this page will refresh automatically.";
    }

    return "When your OBS test reaches BitRiver, your preview will appear here so you can confirm everything before sharing.";
  }, [previewPending, previewReady, testStreamStatus.key]);

  const handleTitleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!titleDraft.trim()) {
      setTitleError("Stream title cannot be empty");
      setTitleSaved(false);
      return;
    }
    try {
      setSavingTitle(true);
      setTitleError(undefined);
      setTitleSaved(false);
      const updated = await updateChannel(channelId, { title: titleDraft.trim() });
      setManagedChannel((prev) => (prev ? { ...prev, ...updated } : updated));
      await reload(true);
      setTitleDraft(updated.title);
      setTitleSaved(true);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unable to update stream title";
      setTitleError(message);
    } finally {
      setSavingTitle(false);
    }
  };

  const handleCopyKey = async () => {
    if (!managedChannel?.streamKey || !isChannelOwner) {
      return;
    }
    try {
      await navigator.clipboard.writeText(managedChannel.streamKey);
      setStreamKeyCopyMessage("Copied");
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unable to copy";
      setStreamKeyCopyMessage(message);
    }
  };

  const handleCopyIngestEndpoint = async (endpoint: string) => {
    try {
      await navigator.clipboard.writeText(endpoint);
      setCopiedIngestEndpoint(endpoint);
      setIngestCopyMessage(`Copied ${describeEndpoint(endpoint, ingestEndpoints.indexOf(endpoint))}`);
    } catch (err) {
      setCopiedIngestEndpoint(undefined);
      const message = err instanceof Error ? err.message : "Unable to copy";
      setIngestCopyMessage(message);
    }
  };

  const handleCopyObsSettings = async () => {
    if (!isChannelOwner) {
      return;
    }
    try {
      await navigator.clipboard.writeText(obsSettingsBlock);
      setObsSettingsCopyMessage("Copied OBS settings");
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unable to copy";
      setObsSettingsCopyMessage(message);
    }
  };

  const handleCopyViewerLink = async () => {
    try {
      await navigator.clipboard.writeText(viewerPageHref);
      setViewerLinkCopyMessage("Copied viewer link");
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unable to copy";
      setViewerLinkCopyMessage(message);
    }
  };

  if (loading) {
    return <section className="surface">Loading channel...</section>;
  }

  if (error) {
    return (
      <section className="surface stack">
        <h2>Unable to load channel</h2>
        <p className="error">{error}</p>
        <Button
          variant="secondary"
          onClick={() => {
            void reload(false);
          }}
        >
          Try again
        </Button>
      </section>
    );
  }

  if (!playback) {
    return (
      <section className="surface stack">
        <h2>Channel not available</h2>
        <p className="muted">We could not find channel details for this dashboard.</p>
      </section>
    );
  }

  return (
    <div className="stack" style={{ gap: "1.5rem" }}>
      <header className="stack" style={{ gap: "0.75rem" }}>
        <h2>Go live</h2>
        <p className="muted">
          Move through this setup from top to bottom: confirm the channel, copy the OBS details, start a test stream,
          check the preview, then share your viewer page.
        </p>
      </header>

      <Card aria-labelledby="channel-section-heading">
        <CardHeader>
          <h3 id="channel-section-heading">1) Channel</h3>
          <p className="muted">Confirm which channel you are about to stream from before you copy anything into OBS.</p>
        </CardHeader>
        <CardBody style={{ gap: "1rem" }}>
          <div className="stack" style={{ gap: "0.5rem" }}>
            <label htmlFor="current-channel-name">Current channel</label>
            <input id="current-channel-name" aria-label="Current channel" readOnly value={currentChannelTitle} />
            <p className="muted">Category: {currentChannelCategory}</p>
          </div>

          {managedChannels.length > 1 ? (
            <div className="stack" style={{ gap: "0.5rem", maxWidth: "24rem" }}>
              <label htmlFor="channel-selector">Switch channel</label>
              <select id="channel-selector" value={channelId} onChange={handleChannelChange}>
                {managedChannels.map((channel) => (
                  <option key={channel.id} value={channel.id}>
                    {channel.title}
                  </option>
                ))}
              </select>
            </div>
          ) : (
            <p className="muted">This is the managed channel currently connected to your account.</p>
          )}

          <form className="stack" style={{ gap: "0.5rem" }} onSubmit={handleTitleSubmit}>
            <div
              className="cluster"
              style={{ justifyContent: "space-between", alignItems: "flex-end", gap: "0.75rem", flexWrap: "wrap" }}
            >
              <div className="stack" style={{ gap: "0.25rem", flex: "1 1 18rem" }}>
                <label className="muted" htmlFor="stream-title-input">
                  Stream title
                </label>
                <input
                  id="stream-title-input"
                  name="streamTitle"
                  value={titleDraft}
                  onChange={(event) => {
                    setTitleDraft(event.target.value);
                    setTitleSaved(false);
                  }}
                  placeholder="What are you streaming today?"
                />
              </div>
              <Button
                type="submit"
                disabled={savingTitle || !titleDraft.trim() || titleDraft.trim() === playback.channel.title}
              >
                {savingTitle ? "Saving..." : "Save title"}
              </Button>
            </div>
            {titleError ? (
              <InlineAlert role="alert">{titleError}</InlineAlert>
            ) : null}
            {titleSaved && !titleError ? <p className="muted">Stream title updated.</p> : null}
          </form>

          {managedLoading ? <p className="muted">Loading channel details...</p> : null}
          {managedError ? <InlineAlert>{managedError}</InlineAlert> : null}
        </CardBody>
      </Card>

      <Card aria-labelledby="obs-setup-heading">
        <CardHeader>
          <h3 id="obs-setup-heading">2) OBS Setup</h3>
          <p className="muted">Paste these exact values into OBS - Settings - Stream. Keep your stream key hidden unless you need to copy it.</p>
        </CardHeader>
        <CardBody style={{ gap: "1rem" }}>
          <div className="stack" style={{ gap: "0.5rem" }}>
            <label htmlFor="preferred-ingest-url">Preferred ingest URL</label>
            {managedLoading ? (
              <p className="muted">Loading ingest configuration...</p>
            ) : preferredIngestEndpoint ? (
              <>
                <div className="cluster" style={{ gap: "0.5rem", flexWrap: "wrap" }}>
                  <input
                    id="preferred-ingest-url"
                    aria-label="Preferred ingest URL"
                    readOnly
                    value={preferredIngestEndpoint}
                  />
                  <Button
                    variant="secondary"
                    data-testid="copy-preferred-ingest-endpoint"
                    onClick={() => {
                      void handleCopyIngestEndpoint(preferredIngestEndpoint);
                    }}
                  >
                    {copiedIngestEndpoint === preferredIngestEndpoint ? "Copied" : "Copy URL"}
                  </Button>
                </div>
                <p className="muted">Use this as the OBS server URL.</p>
              </>
            ) : (
              <p className="muted">Ingest endpoints are not configured yet.</p>
            )}
            {ingestCopyMessage ? (
              <p className={ingestCopyMessage.startsWith("Copied") ? "muted" : "error"}>{ingestCopyMessage}</p>
            ) : null}
          </div>

          <div className="stack" style={{ gap: "0.5rem" }}>
            <label htmlFor="stream-key">Stream key</label>
            {authLoading || managedLoading ? (
              <p className="muted">Verifying channel ownership...</p>
            ) : isChannelOwner ? (
              <>
                <input
                  id="stream-key"
                  aria-label="Stream key"
                  type="text"
                  readOnly
                  value={streamKeyVisible ? managedChannel?.streamKey ?? "" : MASKED_STREAM_KEY}
                />
                <div className="cluster" style={{ gap: "0.5rem", flexWrap: "wrap" }}>
                  <Button
                    variant="secondary"
                    aria-pressed={streamKeyVisible}
                    onClick={() => {
                      setStreamKeyVisible((prev) => !prev);
                      setStreamKeyCopyMessage(undefined);
                    }}
                  >
                    {streamKeyVisible ? "Hide" : "Reveal"}
                  </Button>
                  <Button
                    variant="secondary"
                    onClick={() => {
                      void handleCopyKey();
                    }}
                  >
                    Copy key
                  </Button>
                </div>
                {streamKeyCopyMessage ? (
                  <p className={streamKeyCopyMessage === "Copied" ? "muted" : "error"}>{streamKeyCopyMessage}</p>
                ) : null}
              </>
            ) : (
              <p className="muted">Sign in as the channel owner to reveal or copy the stream key.</p>
            )}
          </div>

          <div className="stack" style={{ gap: "0.5rem" }}>
            <div className="cluster" style={{ justifyContent: "space-between", gap: "0.75rem", flexWrap: "wrap" }}>
              <label htmlFor="obs-settings-block">OBS settings block</label>
              <Button
                variant="secondary"
                data-testid="copy-obs-settings"
                onClick={() => {
                  void handleCopyObsSettings();
                }}
                disabled={!isChannelOwner}
              >
                Copy OBS settings
              </Button>
            </div>
            <textarea
              id="obs-settings-block"
              aria-label="OBS settings block"
              readOnly
              rows={4}
              value={obsSettingsBlock}
              style={{ fontFamily: "ui-monospace, SFMono-Regular, Menlo, Consolas, monospace", resize: "vertical" }}
            />
            <p className="muted">Use `Service: Custom`, then paste the Server and Stream Key values into OBS.</p>
            {obsSettingsCopyMessage ? (
              <p className={obsSettingsCopyMessage.startsWith("Copied") ? "muted" : "error"}>
                {obsSettingsCopyMessage}
              </p>
            ) : null}
          </div>

          {ingestEndpoints.length > 1 ? (
            <details>
              <summary>Show all ingest URLs</summary>
              <div className="stack" style={{ gap: "0.75rem", marginTop: "0.75rem" }}>
                {ingestEndpoints.map((endpoint, index) => (
                  <div key={endpoint} className="stack" style={{ gap: "0.35rem" }}>
                    <label htmlFor={`ingest-endpoint-${index}`}>{describeEndpoint(endpoint, index)}</label>
                    <div className="cluster" style={{ gap: "0.5rem", flexWrap: "wrap" }}>
                      <input id={`ingest-endpoint-${index}`} readOnly value={endpoint} />
                      <Button
                        variant="secondary"
                        data-testid={`copy-ingest-endpoint-${index}`}
                        onClick={() => {
                          void handleCopyIngestEndpoint(endpoint);
                        }}
                      >
                        {copiedIngestEndpoint === endpoint ? "Copied" : "Copy URL"}
                      </Button>
                    </div>
                  </div>
                ))}
              </div>
            </details>
          ) : null}
        </CardBody>
      </Card>

      <Card aria-labelledby="test-stream-heading">
        <CardHeader>
          <h3 id="test-stream-heading">3) Test Stream</h3>
          <p className="muted">Start streaming from OBS. This page refreshes the current live signals every 4 seconds.</p>
        </CardHeader>
        <CardBody style={{ gap: "1rem" }}>
          <div className="surface surface--empty stack" data-testid="test-stream-status-card">
            <div className="cluster" style={{ gap: "0.5rem", flexWrap: "wrap" }}>
              <span className={testStreamStatus.badgeClassName}>{testStreamStatus.label}</span>
              <span className="muted">Last checked {formatTimestamp(testStreamUpdatedAt)}</span>
            </div>
            <p className="muted">{testStreamStatus.instructions}</p>
            {testStreamStatus.reason ? <p className="muted">Signal note: {testStreamStatus.reason}</p> : null}
          </div>

          <div className="cluster" style={{ gap: "0.5rem", flexWrap: "wrap" }}>
            <Button
              variant="secondary"
              onClick={() => {
                void refreshNow();
              }}
            >
              Refresh now
            </Button>
            {latestSession ? (
              <span className="muted">Latest ingest started {formatTimestamp(latestSession.startedAt)}</span>
            ) : (
              <span className="muted">No recent ingest session detected yet.</span>
            )}
          </div>

          {sessionError ? <InlineAlert>{sessionError}</InlineAlert> : null}

          <details>
            <summary>Common issues</summary>
            <ul className="stack muted" style={{ gap: "0.35rem", paddingLeft: "1.25rem", marginTop: "0.5rem" }}>
              <li>Paste the server URL from this page directly into OBS to avoid endpoint mismatches.</li>
              <li>Copy the latest stream key here in case an older key was rotated.</li>
              <li>Keep OBS streaming for a few extra seconds if the status shows reconnecting while the preview warms up.</li>
              <li>If the status stays offline, double-check that OBS is set to RTMP and that the selected channel matches this page.</li>
            </ul>
          </details>
        </CardBody>
      </Card>

      <Card
        aria-labelledby="preview-heading"
        className={previewReady && testStreamStatus.key === "live" ? "surface--glow" : undefined}
      >
        <CardHeader>
          <h3 id="preview-heading">4) Preview</h3>
          <p className="muted">{previewMessage}</p>
        </CardHeader>
        <CardBody style={{ gap: "1rem" }}>
          {latestSession ? (
            <p className="muted">Current session started {formatTimestamp(latestSession.startedAt)}</p>
          ) : null}
          <Player
            playback={playback.playback}
            channelId={channelId}
            live={playback.live}
            liveState={playback.channel.liveState}
          />
        </CardBody>
      </Card>

      <Card aria-labelledby="share-heading">
        <CardHeader>
          <h3 id="share-heading">5) Share</h3>
          <p className="muted">Once the preview looks right, copy your viewer link and open the public page that viewers will use.</p>
        </CardHeader>
        <CardBody style={{ gap: "1rem" }}>
          <div className="stack" style={{ gap: "0.5rem" }}>
            <label htmlFor="viewer-link">Viewer link</label>
            <input id="viewer-link" aria-label="Viewer link" readOnly value={viewerPageHref} />
          </div>

          <div className="cluster" style={{ gap: "0.5rem", flexWrap: "wrap" }}>
            <Button
              variant="secondary"
              data-testid="copy-viewer-link"
              onClick={() => {
                void handleCopyViewerLink();
              }}
            >
              Copy viewer link
            </Button>
            <a href={viewerPageHref} className={buttonClassName("secondary")} target="_blank" rel="noreferrer">
              Open viewer
            </a>
          </div>

          {viewerLinkCopyMessage ? (
            <p className={viewerLinkCopyMessage.startsWith("Copied") ? "muted" : "error"}>{viewerLinkCopyMessage}</p>
          ) : null}
        </CardBody>
      </Card>
    </div>
  );
}
