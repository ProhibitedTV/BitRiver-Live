"use client";

import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { ChannelStudioNav } from "../../../../components/ChannelStudioNav";
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

const MASKED_STREAM_KEY = "********";

type ScheduleDraft = {
  id: string;
  title: string;
  startsAt: string;
  durationMinutes: string;
  description: string;
};

const emptyScheduleDraft: ScheduleDraft = {
  id: "",
  title: "",
  startsAt: "",
  durationMinutes: "60",
  description: "",
};

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

function toDateTimeLocalValue(timestamp?: string) {
  if (!timestamp) {
    return "";
  }
  const date = new Date(timestamp);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  const localDate = new Date(date.getTime() - date.getTimezoneOffset() * 60000);
  return localDate.toISOString().slice(0, 16);
}

function fromDateTimeLocalValue(value: string) {
  if (!value) {
    return undefined;
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return undefined;
  }
  return date.toISOString();
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
  const [scheduleDraft, setScheduleDraft] = useState<ScheduleDraft>(emptyScheduleDraft);
  const [savingSchedule, setSavingSchedule] = useState(false);
  const [scheduleError, setScheduleError] = useState<string | undefined>();
  const [scheduleSaved, setScheduleSaved] = useState(false);
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
      setSessionError(err instanceof Error ? err.message : "Unable to load ingest details");
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
      setManagedError(err instanceof Error ? err.message : "Unable to load channel settings");
    } finally {
      setManagedLoading(false);
    }
  }, [channelId]);

  const refreshNow = useCallback(async () => {
    await Promise.allSettled([reload(true), loadSessions(), loadManagedChannel()]);
    setTestStreamUpdatedAt(new Date().toISOString());
  }, [loadManagedChannel, loadSessions, reload]);

  const primaryScheduleEntry = useMemo(
    () => (managedChannel?.schedule ?? playback?.channel.schedule ?? [])[0],
    [managedChannel?.schedule, playback?.channel.schedule],
  );
  const scheduleDraftSeed = useMemo<ScheduleDraft>(
    () => ({
      id: primaryScheduleEntry?.id ?? "",
      title: primaryScheduleEntry?.title ?? playback?.channel.title ?? "",
      startsAt: toDateTimeLocalValue(primaryScheduleEntry?.startsAt),
      durationMinutes: String(primaryScheduleEntry?.durationMinutes ?? 60),
      description: primaryScheduleEntry?.description ?? "",
    }),
    [
      playback?.channel.title,
      primaryScheduleEntry?.description,
      primaryScheduleEntry?.durationMinutes,
      primaryScheduleEntry?.id,
      primaryScheduleEntry?.startsAt,
      primaryScheduleEntry?.title,
    ],
  );

  useEffect(() => {
    void refreshNow();
  }, [refreshNow]);

  useEffect(() => {
    setTitleDraft(playback?.channel.title ?? "");
    setTitleSaved(false);
    setTitleError(undefined);
  }, [playback?.channel.title]);

  useEffect(() => {
    setScheduleDraft(scheduleDraftSeed);
    setScheduleError(undefined);
  }, [scheduleDraftSeed]);

  useEffect(() => {
    setScheduleSaved(false);
    setScheduleError(undefined);
  }, [channelId]);

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

    const sorted = [...sessions].sort((a, b) => new Date(b.startedAt).getTime() - new Date(a.startedAt).getTime());
    return sorted.find((session) => !session.endedAt) ?? sorted[0];
  }, [sessions]);

  const isChannelOwner = useMemo(
    () => Boolean(managedChannel && user && managedChannel.ownerId === user.id),
    [managedChannel, user],
  );

  const ingestEndpoints = useMemo(() => managedChannel?.ingestEndpoints ?? [], [managedChannel?.ingestEndpoints]);
  const preferredIngestEndpoint = useMemo(() => getPreferredIngestEndpoint(ingestEndpoints), [ingestEndpoints]);
  const obsSettingsBlock = useMemo(
    () => buildObsSettingsBlock(preferredIngestEndpoint, isChannelOwner ? managedChannel?.streamKey : undefined, streamKeyVisible),
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
      return "Preview is live. Check video and audio, then share the channel.";
    }
    if (previewPending) {
      return "BitRiver is receiving your stream. Keep OBS running while the preview starts.";
    }
    return "Start streaming in OBS. The preview will appear here automatically.";
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
      setTitleError(err instanceof Error ? err.message : "Unable to update stream title");
    } finally {
      setSavingTitle(false);
    }
  };

  const handleScheduleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const title = scheduleDraft.title.trim();
    const startsAt = fromDateTimeLocalValue(scheduleDraft.startsAt);
    const durationMinutes = Number.parseInt(scheduleDraft.durationMinutes, 10);

    if (!title) {
      setScheduleError("Scheduled title cannot be empty");
      setScheduleSaved(false);
      return;
    }
    if (!startsAt) {
      setScheduleError("Start time is required");
      setScheduleSaved(false);
      return;
    }
    if (!Number.isFinite(durationMinutes) || durationMinutes <= 0) {
      setScheduleError("Duration must be greater than zero");
      setScheduleSaved(false);
      return;
    }

    try {
      setSavingSchedule(true);
      setScheduleError(undefined);
      setScheduleSaved(false);
      const description = scheduleDraft.description.trim();
      const updated = await updateChannel(channelId, {
        schedule: [
          {
            ...(scheduleDraft.id ? { id: scheduleDraft.id } : {}),
            title,
            startsAt,
            durationMinutes,
            ...(description ? { description } : {}),
          },
        ],
      });
      setManagedChannel((prev) => (prev ? { ...prev, ...updated } : updated));
      await reload(true);
      const savedEntry = updated.schedule?.[0];
      setScheduleDraft({
        id: savedEntry?.id ?? "",
        title: savedEntry?.title ?? updated.title,
        startsAt: toDateTimeLocalValue(savedEntry?.startsAt),
        durationMinutes: String(savedEntry?.durationMinutes ?? 60),
        description: savedEntry?.description ?? "",
      });
      setScheduleSaved(true);
    } catch (err) {
      setScheduleError(err instanceof Error ? err.message : "Unable to update stream schedule");
    } finally {
      setSavingSchedule(false);
    }
  };

  const handleClearSchedule = async () => {
    try {
      setSavingSchedule(true);
      setScheduleError(undefined);
      setScheduleSaved(false);
      const updated = await updateChannel(channelId, { schedule: [] });
      setManagedChannel((prev) => (prev ? { ...prev, ...updated } : updated));
      await reload(true);
      setScheduleDraft({ ...emptyScheduleDraft, title: updated.title });
      setScheduleSaved(true);
    } catch (err) {
      setScheduleError(err instanceof Error ? err.message : "Unable to clear stream schedule");
    } finally {
      setSavingSchedule(false);
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
      setStreamKeyCopyMessage(err instanceof Error ? err.message : "Unable to copy");
    }
  };

  const handleCopyIngestEndpoint = async (endpoint: string) => {
    try {
      await navigator.clipboard.writeText(endpoint);
      setCopiedIngestEndpoint(endpoint);
      setIngestCopyMessage(`Copied ${describeEndpoint(endpoint, ingestEndpoints.indexOf(endpoint))}`);
    } catch (err) {
      setCopiedIngestEndpoint(undefined);
      setIngestCopyMessage(err instanceof Error ? err.message : "Unable to copy");
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
      setObsSettingsCopyMessage(err instanceof Error ? err.message : "Unable to copy");
    }
  };

  const handleCopyViewerLink = async () => {
    try {
      await navigator.clipboard.writeText(viewerPageHref);
      setViewerLinkCopyMessage("Copied viewer link");
    } catch (err) {
      setViewerLinkCopyMessage(err instanceof Error ? err.message : "Unable to copy");
    }
  };

  if (loading) {
    return <section className="surface workspace-card">Loading channel...</section>;
  }

  if (error) {
    return (
      <section className="surface workspace-card">
        <div className="workspace-card__header">
          <h2>Unable to load channel</h2>
          <p className="error">{error}</p>
        </div>
        <div className="workspace-card__actions">
          <Button
            variant="secondary"
            onClick={() => {
              void reload(false);
            }}
          >
            Try again
          </Button>
        </div>
      </section>
    );
  }

  if (!playback) {
    return (
      <section className="surface workspace-card">
        <div className="workspace-card__header">
          <h2>Channel not available</h2>
          <p className="muted">We could not find channel details for this dashboard.</p>
        </div>
      </section>
    );
  }

  return (
    <div className="channel-studio-workspace">
      <ChannelStudioNav
        channelId={channelId}
        channelTitle={currentChannelTitle || playback.channel.title}
        liveState={playback.channel.liveState}
        activeTool="live"
        description="Confirm the channel, copy OBS settings, check the live preview, update schedule, and share the same public channel page viewers use."
      />

      <Card className="workspace-card step-card" aria-labelledby="channel-section-heading">
        <CardHeader className="workspace-card__header">
          <h3 id="channel-section-heading">1) Channel</h3>
          <p className="muted">Confirm the channel and title viewers should see before you open OBS.</p>
        </CardHeader>
        <CardBody className="creator-live__section">
          <label className="input-stack" htmlFor="current-channel-name">
            <span>Current channel</span>
            <input id="current-channel-name" aria-label="Current channel" readOnly value={currentChannelTitle} />
          </label>
          <p className="muted">Category: {currentChannelCategory}</p>

          {managedChannels.length > 1 ? (
            <label className="input-stack" htmlFor="channel-selector">
              <span>Switch channel</span>
              <select id="channel-selector" aria-label="Switch channel" value={channelId} onChange={handleChannelChange}>
                {managedChannels.map((channel) => (
                  <option key={channel.id} value={channel.id}>
                    {channel.title}
                  </option>
                ))}
              </select>
            </label>
          ) : (
            <p className="muted">This is the managed channel currently connected to your account.</p>
          )}

          <form className="creator-live__section" onSubmit={handleTitleSubmit}>
            <div className="creator-live__split">
              <label className="input-stack" htmlFor="stream-title-input">
                <span className="muted">Stream title</span>
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
              </label>
              <Button type="submit" disabled={savingTitle || !titleDraft.trim() || titleDraft.trim() === playback.channel.title}>
                {savingTitle ? "Saving..." : "Save title"}
              </Button>
            </div>
            {titleError ? <InlineAlert role="alert">{titleError}</InlineAlert> : null}
            {titleSaved && !titleError ? <p className="success">Stream title updated.</p> : null}
          </form>

          <form
            id="channel-schedule"
            className="creator-live__section"
            aria-label="Update stream schedule"
            onSubmit={handleScheduleSubmit}
          >
            <h4 className="creator-live__subheading">Upcoming stream</h4>
            <div className="creator-live__schedule-grid">
              <label className="input-stack" htmlFor="schedule-title-input">
                <span className="muted">Scheduled title</span>
                <input
                  id="schedule-title-input"
                  name="scheduleTitle"
                  value={scheduleDraft.title}
                  onChange={(event) => {
                    setScheduleDraft((prev) => ({ ...prev, title: event.target.value }));
                    setScheduleSaved(false);
                  }}
                  placeholder="Friday night stream"
                />
              </label>
              <label className="input-stack" htmlFor="schedule-start-input">
                <span className="muted">Start time</span>
                <input
                  id="schedule-start-input"
                  name="scheduleStart"
                  type="datetime-local"
                  value={scheduleDraft.startsAt}
                  onChange={(event) => {
                    setScheduleDraft((prev) => ({ ...prev, startsAt: event.target.value }));
                    setScheduleSaved(false);
                  }}
                />
              </label>
              <label className="input-stack" htmlFor="schedule-duration-input">
                <span className="muted">Duration minutes</span>
                <input
                  id="schedule-duration-input"
                  name="scheduleDuration"
                  type="number"
                  min={1}
                  step={15}
                  value={scheduleDraft.durationMinutes}
                  onChange={(event) => {
                    setScheduleDraft((prev) => ({ ...prev, durationMinutes: event.target.value }));
                    setScheduleSaved(false);
                  }}
                />
              </label>
              <label className="input-stack creator-live__schedule-description" htmlFor="schedule-description-input">
                <span className="muted">Description</span>
                <textarea
                  id="schedule-description-input"
                  name="scheduleDescription"
                  rows={3}
                  value={scheduleDraft.description}
                  onChange={(event) => {
                    setScheduleDraft((prev) => ({ ...prev, description: event.target.value }));
                    setScheduleSaved(false);
                  }}
                  placeholder="Optional notes for viewers"
                />
              </label>
            </div>
            <div className="workspace-card__actions">
              <Button type="submit" disabled={savingSchedule}>
                {savingSchedule ? "Saving..." : "Save schedule"}
              </Button>
              <Button type="button" variant="secondary" disabled={savingSchedule} onClick={() => void handleClearSchedule()}>
                Clear schedule
              </Button>
            </div>
            {scheduleError ? <InlineAlert role="alert">{scheduleError}</InlineAlert> : null}
            {scheduleSaved && !scheduleError ? <p className="success">Schedule updated.</p> : null}
          </form>

          {managedLoading ? <p className="muted">Loading channel details...</p> : null}
          {managedError ? <InlineAlert>{managedError}</InlineAlert> : null}
        </CardBody>
      </Card>

      <Card className="workspace-card step-card" aria-labelledby="obs-setup-heading">
        <CardHeader className="workspace-card__header">
          <h3 id="obs-setup-heading">2) Stream settings</h3>
          <p className="muted">Use these values in OBS &gt; Settings &gt; Stream.</p>
        </CardHeader>
        <CardBody className="creator-live__section">
          <div className="creator-live__field-group">
            <label className="input-stack" htmlFor="preferred-ingest-url">
              <span>Preferred ingest URL</span>
              {managedLoading ? (
                <p className="muted">Loading ingest configuration...</p>
              ) : preferredIngestEndpoint ? (
                <>
                  <input id="preferred-ingest-url" aria-label="Preferred ingest URL" readOnly value={preferredIngestEndpoint} />
                  <div className="workspace-card__actions">
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
                </>
              ) : (
                <p className="muted">Ingest endpoints are not configured yet.</p>
              )}
            </label>
            <p className="muted">Use this as the OBS server URL.</p>
            {ingestCopyMessage ? (
              <p className={ingestCopyMessage.startsWith("Copied") ? "muted" : "error"}>{ingestCopyMessage}</p>
            ) : null}
          </div>

          <div className="creator-live__field-group">
            <label className="input-stack" htmlFor="stream-key">
              <span>Stream key</span>
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
                  <div className="workspace-card__actions">
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
                </>
              ) : (
                <p className="muted">Sign in as the channel owner to reveal or copy the stream key.</p>
              )}
            </label>
            {streamKeyCopyMessage ? (
              <p className={streamKeyCopyMessage === "Copied" ? "muted" : "error"}>{streamKeyCopyMessage}</p>
            ) : null}
          </div>

          <div className="state-panel">
            <strong>OBS</strong>
            <p className="muted">Service: Custom</p>
            <p className="muted">Server: {preferredIngestEndpoint ?? "Not available yet"}</p>
            <p className="muted">Stream key: reveal or copy it above when you need it.</p>
          </div>

          <div className="workspace-card__actions">
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
          <p className="muted">Paste the copied settings into OBS and start streaming.</p>
          {obsSettingsCopyMessage ? (
            <p className={obsSettingsCopyMessage.startsWith("Copied") ? "muted" : "error"}>{obsSettingsCopyMessage}</p>
          ) : null}
        </CardBody>
      </Card>

      <Card className="workspace-card step-card" aria-labelledby="test-stream-heading">
        <CardHeader className="workspace-card__header">
          <h3 id="test-stream-heading">3) Go live</h3>
          <p className="muted">Start the stream in OBS. This page checks the signal every 4 seconds and shows the preview as soon as playback is ready.</p>
        </CardHeader>
        <CardBody className="creator-live__section">
          <div className="creator-live__signal-card surface--empty" data-testid="test-stream-status-card">
            <div className="eyebrow-row">
              <span className={testStreamStatus.badgeClassName}>{testStreamStatus.label}</span>
              <span className="muted">Last checked {formatTimestamp(testStreamUpdatedAt)}</span>
            </div>
            <p className="muted">{testStreamStatus.instructions}</p>
            {testStreamStatus.reason ? <p className="muted">Signal note: {testStreamStatus.reason}</p> : null}
          </div>

          <div className="workspace-card__actions">
            <Button
              variant="secondary"
              onClick={() => {
                void refreshNow();
              }}
            >
              Refresh now
            </Button>
            {latestSession ? (
              <span className="muted">Current session started {formatTimestamp(latestSession.startedAt)}</span>
            ) : (
              <span className="muted">No recent ingest session detected yet.</span>
            )}
          </div>

          {sessionError ? <InlineAlert>{sessionError}</InlineAlert> : null}

          <div className="stack stack--sm">
            <p className="muted">{previewMessage}</p>
            <Player playback={playback.playback} channelId={channelId} live={playback.live} liveState={playback.channel.liveState} />
          </div>
        </CardBody>
      </Card>

      <Card id="channel-share" className="workspace-card step-card" aria-labelledby="share-heading">
        <CardHeader className="workspace-card__header">
          <h3 id="share-heading">4) Share</h3>
          <p className="muted">When the preview looks right, copy the viewer link and open the same page your audience will use.</p>
        </CardHeader>
        <CardBody className="creator-live__section">
          <label className="input-stack" htmlFor="viewer-link">
            <span>Viewer link</span>
            <input id="viewer-link" aria-label="Viewer link" readOnly value={viewerPageHref} />
          </label>

          <div className="workspace-card__actions">
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
