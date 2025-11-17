"use client";

import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { Player } from "../../../../components/Player";
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

export default function CreatorLivePage() {
  const { playback, loading, error, channelId, reload } = useCreatorChannel();
  const [sessionLoading, setSessionLoading] = useState(true);
  const [sessionError, setSessionError] = useState<string | undefined>();
  const [sessions, setSessions] = useState<StreamSession[]>([]);
  const [managedChannel, setManagedChannel] = useState<ManagedChannel | undefined>();
  const [managedChannels, setManagedChannels] = useState<ManagedChannel[]>([]);
  const [managedError, setManagedError] = useState<string | undefined>();
  const [titleDraft, setTitleDraft] = useState("");
  const [savingTitle, setSavingTitle] = useState(false);
  const [titleError, setTitleError] = useState<string | undefined>();
  const [titleSaved, setTitleSaved] = useState(false);
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
    }
  }, [channelId]);

  useEffect(() => {
    void loadSessions();
  }, [loadSessions]);

  useEffect(() => {
    void loadManagedChannel();
  }, [loadManagedChannel]);

  useEffect(() => {
    setTitleDraft(playback?.channel.title ?? "");
    setTitleSaved(false);
    setTitleError(undefined);
  }, [playback?.channel.title]);

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
          <button type="button" className="secondary-button" onClick={() => { void reload(); void loadSessions(); }}>
            Refresh details
          </button>
          {sessionError ? <span className="error">{sessionError}</span> : null}
          {managedError ? <span className="error">{managedError}</span> : null}
        </div>
      </header>

      <div className="grid two-column">
        <section className="surface stack" aria-labelledby="live-setup-heading">
          <h3 id="live-setup-heading">Stream setup</h3>
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
