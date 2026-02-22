"use client";

import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { Player } from "../../../../components/Player";
import { useAuth } from "../../../../hooks/useAuth";
import { useCreatorChannel } from "../../../../hooks/useCreatorChannel";
import {
  ManagedChannel,
  StreamSession,
  fetchChannelSessions,
  fetchManagedChannels,
  updateChannel,
} from "../../../../lib/viewer-api";

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

type ControlCentreStreamStatus = {
  label: "Idle" | "Ingesting" | "Live" | "Ended" | "Error";
  badgeClassName: string;
  lastTransitionAt?: string;
  reason?: string;
};

function deriveControlCentreStatus(
  liveState: string | undefined,
  currentSessionId: string | undefined,
  latestSession: StreamSession | undefined,
): ControlCentreStreamStatus {
  if (liveState === "starting") {
    return {
      label: "Ingesting",
      badgeClassName: "badge badge--ingesting",
      lastTransitionAt: latestSession?.startedAt,
      reason: "Encoder connected; stream is still provisioning.",
    };
  }

  if (liveState === "live") {
    return {
      label: "Live",
      badgeClassName: "badge badge--live",
      lastTransitionAt: latestSession?.startedAt,
    };
  }

  if (liveState === "offline") {
    if (latestSession?.endedAt) {
      return {
        label: "Ended",
        badgeClassName: "badge badge--ended",
        lastTransitionAt: latestSession.endedAt,
        reason: "Ended normally.",
      };
    }

    if (currentSessionId && !latestSession) {
      return {
        label: "Error",
        badgeClassName: "badge badge--error",
        reason: "Ingest lost before session details were persisted.",
      };
    }

    if (currentSessionId && latestSession && latestSession.id !== currentSessionId) {
      return {
        label: "Error",
        badgeClassName: "badge badge--error",
        lastTransitionAt: latestSession.startedAt,
        reason: "Ingest lost: channel session signal is out of sync.",
      };
    }

    return {
      label: "Idle",
      badgeClassName: "badge badge--muted",
      lastTransitionAt: latestSession?.endedAt,
    };
  }

  // TODO: verify in code if additional persisted live_state values should map to Ended/Error explicitly.
  return {
    label: "Error",
    badgeClassName: "badge badge--error",
    reason: liveState ? `Unexpected stream state: ${liveState}` : "No stream state available",
  };
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
  const [testStreamUpdatedAt, setTestStreamUpdatedAt] = useState<string>(new Date().toISOString());
  const router = useRouter();

  const codeBlockStyle = {
    fontFamily: "monospace",
    backgroundColor: "var(--surface-alt)",
    padding: "0.75rem",
    borderRadius: "0.75rem",
    border: "1px solid var(--border)",
    wordBreak: "break-all" as const,
  };

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
  }, [channelId, managedChannel?.id]);

  useEffect(() => {
    const pollId = window.setInterval(() => {
      void refreshNow();
    }, 4000);

    return () => {
      window.clearInterval(pollId);
    };
  }, [refreshNow]);

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
    [managedChannel, user]
  );

  const ingestEndpoints = useMemo(() => managedChannel?.ingestEndpoints ?? [], [managedChannel?.ingestEndpoints]);
  const preferredIngestEndpoint = useMemo(() => getPreferredIngestEndpoint(ingestEndpoints), [ingestEndpoints]);
  const streamStatus = useMemo(() => {
    if (!playback) {
      return { label: "Idle", badgeClassName: "badge badge--muted" } as ControlCentreStreamStatus;
    }
    return deriveControlCentreStatus(playback.channel.liveState, playback.channel.currentSessionId, latestSession);
  }, [latestSession, playback]);

  const testPanelStatus = useMemo(() => {
    if (!playback?.channel.liveState) {
      return {
        label: "Unknown",
        badgeClassName: "badge badge--muted",
        instructions: "Start streaming in OBS. This page updates automatically.",
      };
    }

    if (playback.channel.liveState === "live") {
      return {
        label: "Live",
        badgeClassName: "badge badge--live",
        instructions: "You're live. Check preview below.",
      };
    }

    if (playback.channel.liveState === "starting") {
      return {
        label: "Reconnecting",
        badgeClassName: "badge badge--ingesting",
        instructions: "Connection interrupted—keep streaming; we'll recover.",
      };
    }

    if (playback.channel.liveState === "offline") {
      return {
        label: "Not live",
        badgeClassName: "badge badge--muted",
        instructions: "Start streaming in OBS. This page updates automatically.",
      };
    }

    return {
      label: "Unknown",
      badgeClassName: "badge badge--error",
      instructions: "Start streaming in OBS. This page updates automatically.",
    };
  }, [playback?.channel.liveState]);

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

  if (loading) {
    return <section className="surface">Loading channel…</section>;
  }

  if (error) {
    return (
      <section className="surface stack">
        <h2>Unable to load channel</h2>
        <p className="error">{error}</p>
        <button type="button" className="secondary-button" onClick={() => { void reload(false); }}>
          Try again
        </button>
      </section>
    );
  }

  if (!playback) {
    return (
      <section className="surface stack">
        <h2>Channel not available</h2>
        <p className="muted">We couldn&apos;t find channel details for this dashboard.</p>
      </section>
    );
  }

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
    if (!managedChannel?.streamKey || !isChannelOwner || ingestEndpoints.length === 0 || !preferredIngestEndpoint) {
      return;
    }
    const streamKeyValue = streamKeyVisible ? managedChannel.streamKey : "[hidden - reveal to copy]";
    const settings = `Server: ${preferredIngestEndpoint}\nStream Key: ${streamKeyValue}`;
    try {
      await navigator.clipboard.writeText(settings);
      setObsSettingsCopyMessage("Copied OBS settings");
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unable to copy";
      setObsSettingsCopyMessage(message);
    }
  };

  return (
    <div className="stack" style={{ gap: "1.5rem" }}>
      <header className="stack">
        <h2>Go live with {playback.channel.title}</h2>
        <p className="muted">
          Configure your encoder with the ingest URL and stream key below, then start sending video to see a live
          preview.
        </p>
        {managedChannels.length > 1 ? (
          <div className="stack" style={{ gap: "0.5rem", maxWidth: "24rem" }}>
            <label className="muted" htmlFor="channel-selector">
              Switch channel
            </label>
            <select id="channel-selector" value={channelId} onChange={handleChannelChange}>
              {managedChannels.map((channel) => (
                <option key={channel.id} value={channel.id}>
                  {channel.title}
                </option>
              ))}
            </select>
          </div>
        ) : null}
        <div className="cluster" style={{ gap: "0.5rem", flexWrap: "wrap" }}>
          <button
            type="button"
            className="secondary-button"
            onClick={() => {
              void reload();
              void loadSessions();
              void loadManagedChannel();
            }}
          >
            Diagnose issues
          </button>
          {sessionError ? <span className="error">{sessionError}</span> : null}
          {managedError ? <span className="error">{managedError}</span> : null}
        </div>
      </header>

      <div className="grid two-column">
        <section className="surface stack" aria-labelledby="obs-setup-heading">
          <h3 id="obs-setup-heading">OBS setup</h3>
          <div className="stack" style={{ gap: "0.5rem" }}>
            <form className="stack" style={{ gap: "0.5rem" }} onSubmit={handleTitleSubmit}>
              <div className="cluster" style={{ justifyContent: "space-between", alignItems: "flex-end", gap: "0.75rem", flexWrap: "wrap" }}>
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
                <button
                  type="submit"
                  className="primary-button"
                  disabled={savingTitle || !titleDraft.trim() || titleDraft.trim() === playback.channel.title}
                >
                  {savingTitle ? "Saving…" : "Save title"}
                </button>
              </div>
              {titleError ? (
                <p className="error" role="alert">{titleError}</p>
              ) : null}
              {titleSaved && !titleError ? <p className="success">Stream title updated</p> : null}
            </form>
            <div className="muted" aria-label="Stream category">
              Category: {formatCategory(playback.channel.category)}
            </div>
            <div className="stack" style={{ gap: "0.75rem" }}>
              <div>
                <p className="muted">Server URL</p>
                {managedLoading ? (
                  <p className="muted">Loading ingest configuration…</p>
                ) : ingestEndpoints.length > 0 ? (
                  <div className="stack" style={{ gap: "0.5rem" }}>
                    <input aria-label="Server URL" readOnly value={preferredIngestEndpoint ?? ""} />
                    <button
                      type="button"
                      className="secondary-button"
                      data-testid="copy-preferred-ingest-endpoint"
                      onClick={() => {
                        if (!preferredIngestEndpoint) {
                          return;
                        }
                        void handleCopyIngestEndpoint(preferredIngestEndpoint);
                      }}
                    >
                      {copiedIngestEndpoint === preferredIngestEndpoint ? "Copied" : "Copy URL"}
                    </button>
                  </div>
                ) : (
                  <p className="muted">Ingest endpoints are not configured yet.</p>
                )}
              </div>

              <div>
                <p className="muted">Stream key</p>
                {authLoading || managedLoading ? (
                  <p className="muted">Verifying channel ownership…</p>
                ) : isChannelOwner ? (
                  <div className="stack" style={{ gap: "0.5rem" }}>
                    <input
                      aria-label="Stream key"
                      type={streamKeyVisible ? "text" : "password"}
                      readOnly
                      value={managedChannel?.streamKey ?? ""}
                    />
                    <div className="cluster" style={{ gap: "0.5rem", flexWrap: "wrap" }}>
                      <button
                        type="button"
                        className="secondary-button"
                        aria-pressed={streamKeyVisible}
                        onClick={() => {
                          setStreamKeyVisible((prev) => !prev);
                          setStreamKeyCopyMessage(undefined);
                        }}
                      >
                        {streamKeyVisible ? "Hide" : "Reveal"}
                      </button>
                      <button
                        type="button"
                        className="secondary-button"
                        onClick={() => {
                          void handleCopyKey();
                        }}
                      >
                        Copy key
                      </button>
                      <button
                        type="button"
                        className="secondary-button"
                        data-testid="copy-obs-settings"
                        onClick={() => {
                          void handleCopyObsSettings();
                        }}
                      >
                        Copy OBS settings
                      </button>
                      {streamKeyCopyMessage ? (
                        <span className={streamKeyCopyMessage === "Copied" ? "success" : "error"}>{streamKeyCopyMessage}</span>
                      ) : null}
                      {obsSettingsCopyMessage ? (
                        <span className={obsSettingsCopyMessage.startsWith("Copied") ? "success" : "error"}>{obsSettingsCopyMessage}</span>
                      ) : null}
                    </div>
                  </div>
                ) : (
                  <p className="muted">Sign in as the channel owner to view the stream key.</p>
                )}
              </div>

              <div>
                <p className="muted">All ingest URLs</p>
                {managedLoading ? (
                  <p className="muted">Loading ingest configuration…</p>
                ) : ingestEndpoints.length > 0 ? (
                  <ul className="stack" style={{ gap: "0.5rem" }}>
                    {ingestEndpoints.map((endpoint, index) => (
                      <li key={endpoint} className="stack" style={{ gap: "0.15rem" }}>
                        <span className="muted">{describeEndpoint(endpoint, index)}</span>
                        <div className="cluster" style={{ gap: "0.5rem", flexWrap: "wrap" }}>
                          <div style={{ ...codeBlockStyle, flex: "1 1 18rem" }}>{endpoint}</div>
                          <button
                            type="button"
                            className="secondary-button"
                            data-testid={`copy-ingest-endpoint-${index}`}
                            onClick={() => {
                              void handleCopyIngestEndpoint(endpoint);
                            }}
                          >
                            {copiedIngestEndpoint === endpoint ? "Copied" : "Copy URL"}
                          </button>
                        </div>
                      </li>
                    ))}
                  </ul>
                ) : (
                  <p className="muted">
                    Ingest endpoints are not configured yet. Check your deployment settings or configuration.
                  </p>
                )}
                {ingestCopyMessage ? (
                  <p className={ingestCopyMessage.startsWith("Copied") ? "success" : "error"}>{ingestCopyMessage}</p>
                ) : null}
              </div>
            </div>
          </div>
        </section>

        <section className="surface stack" aria-labelledby="test-stream-heading">
          <h3 id="test-stream-heading">Test stream</h3>
          <div className="cluster" style={{ gap: "0.5rem", flexWrap: "wrap" }}>
            <span className={testPanelStatus.badgeClassName}>{testPanelStatus.label}</span>
            <span className="muted">Last updated {new Date(testStreamUpdatedAt).toLocaleString()}</span>
          </div>
          <p className="muted">{testPanelStatus.instructions}</p>
          <div className="cluster" style={{ gap: "0.5rem", flexWrap: "wrap" }}>
            <button
              type="button"
              className="secondary-button"
              onClick={() => {
                void refreshNow();
              }}
            >
              Refresh now
            </button>
            {streamStatus.lastTransitionAt ? (
              <span className="muted">Last transition {new Date(streamStatus.lastTransitionAt).toLocaleString()}</span>
            ) : (
              <span className="muted">Last transition unknown</span>
            )}
            {streamStatus.reason ? <span className="muted">Reason: {streamStatus.reason}</span> : null}
          </div>
          <details>
            <summary>Common issues</summary>
            <ul className="stack muted" style={{ gap: "0.35rem", paddingLeft: "1.25rem", marginTop: "0.5rem" }}>
              <li>Server URL mismatch: copy the Server URL from this page and paste it into OBS.</li>
              <li>Wrong stream key: copy the latest key here in case your previous key was rotated.</li>
              <li>Network blocked: some networks block streaming ports, so try a different network if possible.</li>
              <li>Protocol mismatch: this setup expects RTMP, so confirm OBS is sending RTMP to the shown server URL.</li>
            </ul>
          </details>
        </section>

        <section className="surface stack" aria-labelledby="preview-heading">
          <div className="cluster" style={{ justifyContent: "space-between", alignItems: "baseline" }}>
            <h3 id="preview-heading">Stream preview</h3>
            {latestSession ? (
              <span className="muted">Session started {new Date(latestSession.startedAt).toLocaleString()}</span>
            ) : (
              <span className="muted">No active session yet</span>
            )}
          </div>
          <Player playback={playback.playback} channelId={channelId} />
        </section>
      </div>
    </div>
  );
}
