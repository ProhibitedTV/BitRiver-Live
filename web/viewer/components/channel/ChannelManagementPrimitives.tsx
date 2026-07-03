"use client";

import type { ChangeEventHandler, Dispatch, FormEventHandler, SetStateAction } from "react";
import { Player } from "../Player";
import { Button, buttonClassName } from "../ui/Button";
import { Card, CardBody, CardHeader } from "../ui/Card";
import { InlineAlert } from "../ui/InlineAlert";
import type { ChannelPlaybackResponse, ManagedChannel, StreamSession } from "../../lib/viewer-api";

export const MASKED_STREAM_KEY = "********";

export type ScheduleDraft = {
  id: string;
  title: string;
  startsAt: string;
  durationMinutes: string;
  description: string;
};

export const emptyScheduleDraft: ScheduleDraft = {
  id: "",
  title: "",
  startsAt: "",
  durationMinutes: "60",
  description: "",
};

export function formatCategory(category?: string) {
  if (!category) {
    return "Uncategorized";
  }
  return category;
}

export function describeEndpoint(endpoint: string, index: number) {
  if (index === 0) {
    return "Primary ingest";
  }
  if (index === 1) {
    return "Backup ingest";
  }
  return `Ingest ${index + 1}`;
}

export function getPreferredIngestEndpoint(endpoints: string[]) {
  const rtmpEndpoint = endpoints.find((endpoint) => endpoint.toLowerCase().startsWith("rtmp://"));
  return rtmpEndpoint ?? endpoints[0];
}

export function formatTimestamp(timestamp?: string) {
  if (!timestamp) {
    return "Checking now...";
  }
  return new Date(timestamp).toLocaleString();
}

export function toDateTimeLocalValue(timestamp?: string) {
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

export function fromDateTimeLocalValue(value: string) {
  if (!value) {
    return undefined;
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return undefined;
  }
  return date.toISOString();
}

export function buildObsSettingsBlock(preferredIngestEndpoint?: string, streamKey?: string, streamKeyVisible = false) {
  const serverLine = preferredIngestEndpoint ? `Server: ${preferredIngestEndpoint}` : "Server: [not available yet]";
  const streamKeyLine = streamKey
    ? `Stream Key: ${streamKeyVisible ? streamKey : "[hidden - reveal to copy]"}`
    : "Stream Key: [owner access required]";

  return `Service: Custom\n${serverLine}\n${streamKeyLine}`;
}

type GoLiveStatusView = {
  key: "waiting" | "live" | "reconnecting" | "offline-unknown";
  label: string;
  badgeClassName: string;
  instructions: string;
  reason?: string;
};

type ChannelIdentityPanelProps = {
  channelId: string;
  currentChannelTitle: string;
  currentChannelCategory: string;
  managedChannels: ManagedChannel[];
  onChannelChange: ChangeEventHandler<HTMLSelectElement>;
};

export function ChannelIdentityPanel({
  channelId,
  currentChannelTitle,
  currentChannelCategory,
  managedChannels,
  onChannelChange,
}: ChannelIdentityPanelProps) {
  return (
    <>
      <label className="input-stack" htmlFor="current-channel-name">
        <span>Current channel</span>
        <input id="current-channel-name" aria-label="Current channel" readOnly value={currentChannelTitle} />
      </label>
      <p className="muted">Category: {currentChannelCategory}</p>

      {managedChannels.length > 1 ? (
        <label className="input-stack" htmlFor="channel-selector">
          <span>Switch channel</span>
          <select id="channel-selector" aria-label="Switch channel" value={channelId} onChange={onChannelChange}>
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
    </>
  );
}

type StreamTitleEditorProps = {
  titleDraft: string;
  currentTitle: string;
  saving: boolean;
  error?: string;
  saved: boolean;
  onTitleDraftChange: (value: string) => void;
  onSubmit: FormEventHandler<HTMLFormElement>;
};

export function StreamTitleEditor({
  titleDraft,
  currentTitle,
  saving,
  error,
  saved,
  onTitleDraftChange,
  onSubmit,
}: StreamTitleEditorProps) {
  return (
    <form className="creator-live__section" onSubmit={onSubmit}>
      <div className="creator-live__split">
        <label className="input-stack" htmlFor="stream-title-input">
          <span className="muted">Stream title</span>
          <input
            id="stream-title-input"
            name="streamTitle"
            value={titleDraft}
            onChange={(event) => {
              onTitleDraftChange(event.target.value);
            }}
            placeholder="What are you streaming today?"
          />
        </label>
        <Button type="submit" disabled={saving || !titleDraft.trim() || titleDraft.trim() === currentTitle}>
          {saving ? "Saving..." : "Save title"}
        </Button>
      </div>
      {error ? <InlineAlert role="alert">{error}</InlineAlert> : null}
      {saved && !error ? <p className="success">Stream title updated.</p> : null}
    </form>
  );
}

type ScheduleEditorProps = {
  draft: ScheduleDraft;
  saving: boolean;
  error?: string;
  saved: boolean;
  onDraftChange: Dispatch<SetStateAction<ScheduleDraft>>;
  onSubmit: FormEventHandler<HTMLFormElement>;
  onClear: () => void;
};

export function ScheduleEditor({ draft, saving, error, saved, onDraftChange, onSubmit, onClear }: ScheduleEditorProps) {
  return (
    <form id="channel-schedule" className="creator-live__section" aria-label="Update stream schedule" onSubmit={onSubmit}>
      <h4 className="creator-live__subheading">Upcoming stream</h4>
      <div className="creator-live__schedule-grid">
        <label className="input-stack" htmlFor="schedule-title-input">
          <span className="muted">Scheduled title</span>
          <input
            id="schedule-title-input"
            name="scheduleTitle"
            value={draft.title}
            onChange={(event) => {
              onDraftChange((prev) => ({ ...prev, title: event.target.value }));
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
            value={draft.startsAt}
            onChange={(event) => {
              onDraftChange((prev) => ({ ...prev, startsAt: event.target.value }));
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
            value={draft.durationMinutes}
            onChange={(event) => {
              onDraftChange((prev) => ({ ...prev, durationMinutes: event.target.value }));
            }}
          />
        </label>
        <label className="input-stack creator-live__schedule-description" htmlFor="schedule-description-input">
          <span className="muted">Description</span>
          <textarea
            id="schedule-description-input"
            name="scheduleDescription"
            rows={3}
            value={draft.description}
            onChange={(event) => {
              onDraftChange((prev) => ({ ...prev, description: event.target.value }));
            }}
            placeholder="Optional notes for viewers"
          />
        </label>
      </div>
      <div className="workspace-card__actions">
        <Button type="submit" disabled={saving}>
          {saving ? "Saving..." : "Save schedule"}
        </Button>
        <Button type="button" variant="secondary" disabled={saving} onClick={onClear}>
          Clear schedule
        </Button>
      </div>
      {error ? <InlineAlert role="alert">{error}</InlineAlert> : null}
      {saved && !error ? <p className="success">Schedule updated.</p> : null}
    </form>
  );
}

type ChannelManagementStatusProps = {
  managedLoading: boolean;
  managedError?: string;
};

export function ChannelManagementStatus({ managedLoading, managedError }: ChannelManagementStatusProps) {
  return (
    <>
      {managedLoading ? <p className="muted">Loading channel details...</p> : null}
      {managedError ? <InlineAlert>{managedError}</InlineAlert> : null}
    </>
  );
}

type ChannelSetupCardProps = ChannelIdentityPanelProps &
  ChannelManagementStatusProps & {
    titleDraft: string;
    currentTitle: string;
    savingTitle: boolean;
    titleError?: string;
    titleSaved: boolean;
    onTitleDraftChange: (value: string) => void;
    onTitleSubmit: FormEventHandler<HTMLFormElement>;
    scheduleDraft: ScheduleDraft;
    savingSchedule: boolean;
    scheduleError?: string;
    scheduleSaved: boolean;
    onScheduleDraftChange: Dispatch<SetStateAction<ScheduleDraft>>;
    onScheduleSubmit: FormEventHandler<HTMLFormElement>;
    onClearSchedule: () => void;
  };

export function ChannelSetupCard({
  channelId,
  currentChannelTitle,
  currentChannelCategory,
  managedChannels,
  onChannelChange,
  titleDraft,
  currentTitle,
  savingTitle,
  titleError,
  titleSaved,
  onTitleDraftChange,
  onTitleSubmit,
  scheduleDraft,
  savingSchedule,
  scheduleError,
  scheduleSaved,
  onScheduleDraftChange,
  onScheduleSubmit,
  onClearSchedule,
  managedLoading,
  managedError,
}: ChannelSetupCardProps) {
  return (
    <Card className="workspace-card step-card" aria-labelledby="channel-section-heading">
      <CardHeader className="workspace-card__header">
        <h3 id="channel-section-heading">1) Channel</h3>
        <p className="muted">Confirm channel, title, and schedule.</p>
      </CardHeader>
      <CardBody className="creator-live__section">
        <ChannelIdentityPanel
          channelId={channelId}
          currentChannelTitle={currentChannelTitle}
          currentChannelCategory={currentChannelCategory}
          managedChannels={managedChannels}
          onChannelChange={onChannelChange}
        />
        <StreamTitleEditor
          titleDraft={titleDraft}
          currentTitle={currentTitle}
          saving={savingTitle}
          error={titleError}
          saved={titleSaved}
          onTitleDraftChange={onTitleDraftChange}
          onSubmit={onTitleSubmit}
        />
        <ScheduleEditor
          draft={scheduleDraft}
          saving={savingSchedule}
          error={scheduleError}
          saved={scheduleSaved}
          onDraftChange={onScheduleDraftChange}
          onSubmit={onScheduleSubmit}
          onClear={onClearSchedule}
        />
        <ChannelManagementStatus managedLoading={managedLoading} managedError={managedError} />
      </CardBody>
    </Card>
  );
}

type ObsSettingsPanelProps = {
  authLoading: boolean;
  managedLoading: boolean;
  isChannelOwner: boolean;
  streamKeyVisible: boolean;
  streamKey?: string;
  preferredIngestEndpoint?: string;
  copiedIngestEndpoint?: string;
  ingestCopyMessage?: string;
  streamKeyCopyMessage?: string;
  obsSettingsCopyMessage?: string;
  onCopyIngestEndpoint: (endpoint: string) => void;
  onToggleStreamKey: () => void;
  onCopyKey: () => void;
  onCopyObsSettings: () => void;
};

export function ObsSettingsPanel({
  authLoading,
  managedLoading,
  isChannelOwner,
  streamKeyVisible,
  streamKey,
  preferredIngestEndpoint,
  copiedIngestEndpoint,
  ingestCopyMessage,
  streamKeyCopyMessage,
  obsSettingsCopyMessage,
  onCopyIngestEndpoint,
  onToggleStreamKey,
  onCopyKey,
  onCopyObsSettings,
}: ObsSettingsPanelProps) {
  return (
    <Card className="workspace-card step-card" aria-labelledby="obs-setup-heading">
      <CardHeader className="workspace-card__header">
        <h3 id="obs-setup-heading">2) Stream settings</h3>
        <p className="muted">Paste these values into OBS.</p>
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
                      onCopyIngestEndpoint(preferredIngestEndpoint);
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
          {ingestCopyMessage ? <p className={ingestCopyMessage.startsWith("Copied") ? "muted" : "error"}>{ingestCopyMessage}</p> : null}
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
                  value={streamKeyVisible ? streamKey ?? "" : MASKED_STREAM_KEY}
                />
                <div className="workspace-card__actions">
                  <Button variant="secondary" aria-pressed={streamKeyVisible} onClick={onToggleStreamKey}>
                    {streamKeyVisible ? "Hide" : "Reveal"}
                  </Button>
                  <Button variant="secondary" onClick={onCopyKey}>
                    Copy key
                  </Button>
                </div>
              </>
            ) : (
              <p className="muted">Sign in as the channel owner to reveal or copy the stream key.</p>
            )}
          </label>
          {streamKeyCopyMessage ? <p className={streamKeyCopyMessage === "Copied" ? "muted" : "error"}>{streamKeyCopyMessage}</p> : null}
        </div>

        <div className="state-panel">
          <strong>OBS</strong>
          <p className="muted">Service: Custom</p>
          <p className="muted">Server: {preferredIngestEndpoint ?? "Not available yet"}</p>
          <p className="muted">Stream key: reveal or copy above.</p>
        </div>

        <div className="workspace-card__actions">
          <Button variant="secondary" data-testid="copy-obs-settings" onClick={onCopyObsSettings} disabled={!isChannelOwner}>
            Copy OBS settings
          </Button>
        </div>
        <p className="muted">Start streaming after OBS is configured.</p>
        {obsSettingsCopyMessage ? (
          <p className={obsSettingsCopyMessage.startsWith("Copied") ? "muted" : "error"}>{obsSettingsCopyMessage}</p>
        ) : null}
      </CardBody>
    </Card>
  );
}

type LivePreviewPanelProps = {
  channelId: string;
  playback: ChannelPlaybackResponse;
  testStreamStatus: GoLiveStatusView;
  testStreamUpdatedAt?: string;
  latestSession?: StreamSession;
  sessionError?: string;
  previewMessage: string;
  onRefresh: () => void;
};

export function LivePreviewPanel({
  channelId,
  playback,
  testStreamStatus,
  testStreamUpdatedAt,
  latestSession,
  sessionError,
  previewMessage,
  onRefresh,
}: LivePreviewPanelProps) {
  return (
    <Card className="workspace-card step-card" aria-labelledby="test-stream-heading">
      <CardHeader className="workspace-card__header">
        <h3 id="test-stream-heading">3) Go live</h3>
        <p className="muted">Start OBS; preview appears when playback is ready.</p>
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
          <Button variant="secondary" onClick={onRefresh}>
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
  );
}

type ShareLinkPanelProps = {
  viewerPageHref: string;
  viewerLinkCopyMessage?: string;
  onCopyViewerLink: () => void;
};

export function ShareLinkPanel({ viewerPageHref, viewerLinkCopyMessage, onCopyViewerLink }: ShareLinkPanelProps) {
  return (
    <Card id="channel-share" className="workspace-card step-card" aria-labelledby="share-heading">
      <CardHeader className="workspace-card__header">
        <h3 id="share-heading">4) Share</h3>
        <p className="muted">Copy the viewer link.</p>
      </CardHeader>
      <CardBody className="creator-live__section">
        <label className="input-stack" htmlFor="viewer-link">
          <span>Viewer link</span>
          <input id="viewer-link" aria-label="Viewer link" readOnly value={viewerPageHref} />
        </label>

        <div className="workspace-card__actions">
          <Button variant="secondary" data-testid="copy-viewer-link" onClick={onCopyViewerLink}>
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
  );
}
