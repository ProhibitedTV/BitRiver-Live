"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Player } from "../../../../components/Player";
import { useCreatorChannel } from "../../../../hooks/useCreatorChannel";
import {
  ManagedChannel,
  StreamSession,
  fetchChannelSessions,
  fetchManagedChannels,
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

export default function CreatorLivePage() {
  const { playback, loading, error, channelId, reload } = useCreatorChannel();
  const [sessionLoading, setSessionLoading] = useState(true);
  const [sessionError, setSessionError] = useState<string | undefined>();
  const [sessions, setSessions] = useState<StreamSession[]>([]);
  const [managedChannel, setManagedChannel] = useState<ManagedChannel | undefined>();
  const [managedError, setManagedError] = useState<string | undefined>();

  const codeBlockStyle = {
    fontFamily: "monospace",
    backgroundColor: "var(--surface-alt)",
    padding: "0.75rem",
    borderRadius: "0.75rem",
    border: "1px solid var(--border)",
    wordBreak: "break-all" as const,
  };

  const loadSessions = useCallback(async () => {
    setSessionLoading(true);
    setSessionError(undefined);
    try {
      const response = await fetchChannelSessions(channelId);
      setSessions(response ?? []);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unable to load ingest details";
      setSessionError(message);
    } finally {
      setSessionLoading(false);
    }
  }, [channelId]);

  const loadManagedChannel = useCallback(async () => {
    setManagedError(undefined);
    try {
      const channels = await fetchManagedChannels();
      const match = channels.find((channel) => channel.id === channelId);
      setManagedChannel(match);
      if (!match) {
        setManagedError("Channel access unavailable");
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unable to load channel settings";
      setManagedError(message);
    }
  }, [channelId]);

  useEffect(() => {
    void loadSessions();
  }, [loadSessions]);

  useEffect(() => {
    void loadManagedChannel();
  }, [loadManagedChannel]);

  const latestSession = useMemo(() => {
    if (sessions.length === 0) {
      return undefined;
    }
    const sorted = [...sessions].sort(
      (a, b) => new Date(b.startedAt).getTime() - new Date(a.startedAt).getTime(),
    );
    return sorted.find((session) => !session.endedAt) ?? sorted[0];
  }, [sessions]);

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

  const ingestEndpoints = latestSession?.ingestEndpoints ?? [];

  return (
    <div className="stack" style={{ gap: "1.5rem" }}>
      <header className="stack">
        <h2>Go live with {playback.channel.title}</h2>
        <p className="muted">
          Configure your encoder with the ingest URL and stream key below, then start sending video to see a live
          preview.
        </p>
        <div className="cluster" style={{ gap: "0.5rem", flexWrap: "wrap" }}>
          <button type="button" className="secondary-button" onClick={() => { void reload(); void loadSessions(); }}>
            Refresh details
          </button>
          {sessionError ? <span className="error">{sessionError}</span> : null}
          {managedError ? <span className="error">{managedError}</span> : null}
        </div>
      </header>

      <div className="grid two-column">
        <section className="surface stack" aria-labelledby="live-setup-heading">
          <div className="stack" style={{ gap: "0.5rem" }}>
            <div className="cluster" style={{ justifyContent: "space-between", alignItems: "flex-end" }}>
              <div>
                <p className="muted">Stream title</p>
                <h3 id="live-setup-heading">{playback.channel.title}</h3>
              </div>
              <div className="muted" aria-label="Stream category">
                Category: {formatCategory(playback.channel.category)}
              </div>
            </div>
            <div className="stack" style={{ gap: "0.75rem" }}>
              <div>
                <p className="muted">Stream key</p>
                <div style={codeBlockStyle} aria-live="polite">
                  {managedChannel?.streamKey ?? "Unavailable"}
                </div>
              </div>

              <div>
                <p className="muted">Ingest URLs</p>
                {sessionLoading ? (
                  <p className="muted">Loading ingest details…</p>
                ) : ingestEndpoints.length > 0 ? (
                  <ul className="stack" style={{ gap: "0.5rem" }}>
                    {ingestEndpoints.map((endpoint, index) => (
                      <li key={endpoint} className="stack" style={{ gap: "0.15rem" }}>
                        <span className="muted">{describeEndpoint(endpoint, index)}</span>
                        <div style={codeBlockStyle}>{endpoint}</div>
                      </li>
                    ))}
                  </ul>
                ) : (
                  <p className="muted">
                    Start streaming from your encoder to receive ingest connection details for this session.
                  </p>
                )}
              </div>
            </div>
          </div>
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
          <Player playback={playback.playback} />
        </section>
      </div>
    </div>
  );
}
