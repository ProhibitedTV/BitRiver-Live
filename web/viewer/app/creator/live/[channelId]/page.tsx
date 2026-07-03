"use client";

import { ChangeEvent, FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import {
  ChannelSetupCard,
  LivePreviewPanel,
  ObsSettingsPanel,
  ShareLinkPanel,
  buildObsSettingsBlock,
  describeEndpoint,
  emptyScheduleDraft,
  formatCategory,
  fromDateTimeLocalValue,
  getPreferredIngestEndpoint,
  toDateTimeLocalValue,
} from "../../../../components/channel/ChannelManagementPrimitives";
import type { ScheduleDraft } from "../../../../components/channel/ChannelManagementPrimitives";
import { ChannelStudioNav } from "../../../../components/ChannelStudioNav";
import { Button } from "../../../../components/ui/Button";
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

  const handleChannelChange = (event: ChangeEvent<HTMLSelectElement>) => {
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
      return "Preview is live. Check video and audio, then share.";
    }
    if (previewPending) {
      return "Receiving stream. Keep OBS running.";
    }
    return "Start OBS. Preview appears here.";
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
        description="Copy OBS settings, check preview, update schedule, and share."
      />

      <ChannelSetupCard
        channelId={channelId}
        currentChannelTitle={currentChannelTitle}
        currentChannelCategory={currentChannelCategory}
        managedChannels={managedChannels}
        onChannelChange={handleChannelChange}
        titleDraft={titleDraft}
        currentTitle={playback.channel.title}
        savingTitle={savingTitle}
        titleError={titleError}
        titleSaved={titleSaved}
        onTitleDraftChange={(value) => {
          setTitleDraft(value);
          setTitleSaved(false);
        }}
        onTitleSubmit={handleTitleSubmit}
        scheduleDraft={scheduleDraft}
        savingSchedule={savingSchedule}
        scheduleError={scheduleError}
        scheduleSaved={scheduleSaved}
        onScheduleDraftChange={(updater) => {
          setScheduleDraft(updater);
          setScheduleSaved(false);
        }}
        onScheduleSubmit={handleScheduleSubmit}
        onClearSchedule={() => {
          void handleClearSchedule();
        }}
        managedLoading={managedLoading}
        managedError={managedError}
      />

      <ObsSettingsPanel
        authLoading={authLoading}
        managedLoading={managedLoading}
        isChannelOwner={isChannelOwner}
        streamKeyVisible={streamKeyVisible}
        streamKey={managedChannel?.streamKey}
        preferredIngestEndpoint={preferredIngestEndpoint}
        copiedIngestEndpoint={copiedIngestEndpoint}
        ingestCopyMessage={ingestCopyMessage}
        streamKeyCopyMessage={streamKeyCopyMessage}
        obsSettingsCopyMessage={obsSettingsCopyMessage}
        onCopyIngestEndpoint={(endpoint) => {
          void handleCopyIngestEndpoint(endpoint);
        }}
        onToggleStreamKey={() => {
          setStreamKeyVisible((prev) => !prev);
          setStreamKeyCopyMessage(undefined);
        }}
        onCopyKey={() => {
          void handleCopyKey();
        }}
        onCopyObsSettings={() => {
          void handleCopyObsSettings();
        }}
      />

      <LivePreviewPanel
        channelId={channelId}
        playback={playback}
        testStreamStatus={testStreamStatus}
        testStreamUpdatedAt={testStreamUpdatedAt}
        latestSession={latestSession}
        sessionError={sessionError}
        previewMessage={previewMessage}
        onRefresh={() => {
          void refreshNow();
        }}
      />

      <ShareLinkPanel
        viewerPageHref={viewerPageHref}
        viewerLinkCopyMessage={viewerLinkCopyMessage}
        onCopyViewerLink={() => {
          void handleCopyViewerLink();
        }}
      />
    </div>
  );
}
