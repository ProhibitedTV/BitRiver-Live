"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useState } from "react";
import { Badge } from "../../../components/ui/Badge";
import { Button, buttonClassName } from "../../../components/ui/Button";
import { Card, CardBody, CardHeader } from "../../../components/ui/Card";
import { useAuth } from "../../../hooks/useAuth";
import { buildViewerPath, buildViewerUrl } from "../../../lib/viewer-links";
import { ManagedChannel, createChannel, fetchChannelPlayback, fetchManagedChannels } from "../../../lib/viewer-api";

type ManualChecks = {
  obsConfigured: boolean;
  viewerLinkShared: boolean;
  vodUploaded: boolean;
};

type ChannelFormState = {
  title: string;
  category: string;
  tags: string;
};

const MANUAL_CHECKS_STORAGE_KEY = "creator-getting-started-manual-checks";

function loadStoredChecks(): ManualChecks {
  if (typeof window === "undefined") {
    return { obsConfigured: false, viewerLinkShared: false, vodUploaded: false };
  }

  try {
    const raw = window.localStorage.getItem(MANUAL_CHECKS_STORAGE_KEY);
    if (!raw) {
      return { obsConfigured: false, viewerLinkShared: false, vodUploaded: false };
    }

    const parsed = JSON.parse(raw) as Partial<ManualChecks>;
    return {
      obsConfigured: Boolean(parsed.obsConfigured),
      viewerLinkShared: Boolean(parsed.viewerLinkShared),
      vodUploaded: Boolean(parsed.vodUploaded),
    };
  } catch {
    return { obsConfigured: false, viewerLinkShared: false, vodUploaded: false };
  }
}

function parseTagInput(input: string) {
  return input
    .split(",")
    .map((tag) => tag.trim())
    .filter(Boolean);
}

export default function CreatorGettingStartedPage() {
  const { user, loading: authLoading, refreshViewer, signIn, signUp } = useAuth();
  const [channels, setChannels] = useState<ManagedChannel[]>([]);
  const [channelsLoading, setChannelsLoading] = useState(false);
  const [channelsError, setChannelsError] = useState<string | undefined>();
  const [selectedChannelId, setSelectedChannelId] = useState<string>("");
  const [channelForm, setChannelForm] = useState<ChannelFormState>({ title: "", category: "", tags: "" });
  const [creatingChannel, setCreatingChannel] = useState(false);
  const [createChannelError, setCreateChannelError] = useState<string | undefined>();
  const [createChannelSuccess, setCreateChannelSuccess] = useState<string | undefined>();
  const [isLive, setIsLive] = useState(false);
  const [liveLoading, setLiveLoading] = useState(false);
  const [liveError, setLiveError] = useState<string | undefined>();
  const [copyMessage, setCopyMessage] = useState<string | undefined>();
  const [manualChecks, setManualChecks] = useState<ManualChecks>(() => loadStoredChecks());

  const loadChannels = useCallback(async () => {
    if (!user) {
      setChannels([]);
      setSelectedChannelId("");
      return;
    }

    setChannelsLoading(true);
    setChannelsError(undefined);
    try {
      const response = await fetchManagedChannels();
      setChannels(response);
      setSelectedChannelId((current) => current || response[0]?.id || "");
    } catch (err) {
      setChannels([]);
      setChannelsError(err instanceof Error ? err.message : "Unable to load channels");
      setSelectedChannelId("");
    } finally {
      setChannelsLoading(false);
    }
  }, [user]);

  useEffect(() => {
    void loadChannels();
  }, [loadChannels]);

  const selectedChannel = useMemo(
    () => channels.find((channel) => channel.id === selectedChannelId) ?? channels[0],
    [channels, selectedChannelId],
  );

  const refreshLiveStatus = useCallback(async () => {
    if (!selectedChannel?.id) {
      setIsLive(false);
      return;
    }

    setLiveLoading(true);
    setLiveError(undefined);
    try {
      const playback = await fetchChannelPlayback(selectedChannel.id);
      setIsLive(Boolean(playback?.live));
    } catch (err) {
      setIsLive(false);
      setLiveError(err instanceof Error ? err.message : "Unable to check live status");
    } finally {
      setLiveLoading(false);
    }
  }, [selectedChannel]);

  useEffect(() => {
    void refreshLiveStatus();
  }, [refreshLiveStatus]);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    window.localStorage.setItem(MANUAL_CHECKS_STORAGE_KEY, JSON.stringify(manualChecks));
  }, [manualChecks]);

  useEffect(() => {
    if (!selectedChannel?.id) {
      return;
    }

    const pollId = window.setInterval(() => {
      void refreshLiveStatus();
    }, 10000);

    return () => {
      window.clearInterval(pollId);
    };
  }, [refreshLiveStatus, selectedChannel?.id]);

  const liveSetupLink = selectedChannel ? `/creator/live/${selectedChannel.id}` : "/creator";
  const uploadsLink = selectedChannel ? `/creator/uploads/${selectedChannel.id}` : "/creator";
  const viewerLink = selectedChannel ? buildViewerPath(`/channels/${selectedChannel.id}`) : buildViewerPath("/browse");
  const hasChannel = Boolean(selectedChannel?.id);
  const needsFirstChannel = Boolean(user) && !channelsLoading && channels.length === 0;

  const handleCreateChannel = useCallback(async () => {
    const title = channelForm.title.trim();
    if (!title) {
      setCreateChannelError("Add a channel name before continuing.");
      setCreateChannelSuccess(undefined);
      return;
    }

    setCreatingChannel(true);
    setCreateChannelError(undefined);
    setCreateChannelSuccess(undefined);
    try {
      const created = await createChannel({
        title,
        category: channelForm.category.trim() || undefined,
        tags: parseTagInput(channelForm.tags),
      });
      setChannels([created]);
      setSelectedChannelId(created.id);
      setChannelForm({ title: "", category: "", tags: "" });
      setCreateChannelSuccess("Channel created. Your live setup is ready below.");
      await refreshViewer();
    } catch (err) {
      setCreateChannelError(err instanceof Error ? err.message : "Unable to create channel");
    } finally {
      setCreatingChannel(false);
    }
  }, [channelForm.category, channelForm.tags, channelForm.title, refreshViewer]);

  const handleCopyViewerLink = useCallback(async () => {
    if (typeof window === "undefined" || !selectedChannel) {
      return;
    }

    try {
      await navigator.clipboard.writeText(buildViewerUrl(`/channels/${selectedChannel.id}`, window.location.origin));
      setCopyMessage("Viewer link copied");
    } catch {
      setCopyMessage("Copy failed. Open the viewer page and copy the URL manually.");
    }
  }, [selectedChannel]);

  const step1Done = hasChannel;
  const step2Done = manualChecks.obsConfigured;
  const step3Done = isLive;
  const step4Done = manualChecks.viewerLinkShared;
  const step5Done = manualChecks.vodUploaded;
  const completedSteps = [step1Done, step2Done, step3Done, step4Done, step5Done].filter(Boolean).length;
  const liveStatusTone: "neutral" | "info" | "success" | "danger" = liveLoading
    ? "info"
    : liveError
      ? "danger"
      : step3Done
        ? "success"
        : "neutral";
  const liveStatusLabel = liveLoading
    ? "Checking"
    : liveError
      ? "Needs attention"
      : step3Done
        ? "Live"
        : hasChannel
          ? "Offline"
          : "Choose a channel";

  return (
    <div className="workspace-shell">
      <section className="workspace-hero">
        <div className="workspace-hero__copy">
          <span className="page-eyebrow">Creator setup</span>
          <h2>Start streaming</h2>
          <p className="muted">Create a channel, copy OBS settings, check the signal, and share the link.</p>
        </div>
        <div className="workspace-hero__actions">
          {hasChannel ? (
            <>
              <Link href={viewerLink} className={buttonClassName("secondary")}>
                Open public preview
              </Link>
              <Link href="/creator" className={buttonClassName("secondary")}>
                Studio
              </Link>
            </>
          ) : (
            <>
              <Link href="/browse" className={buttonClassName("secondary")}>
                Browse live channels
              </Link>
              <Link href="#creator-step-1" className={buttonClassName("secondary")}>
                Start setup
              </Link>
            </>
          )}
        </div>
        <p className="muted">Progress: {completedSteps}/5 steps. Channel: {selectedChannel?.title ?? "none"}. Signal: {step3Done ? "live" : "offline"}.</p>
      </section>

      {!user && !authLoading ? (
        <Card className="workspace-card">
          <CardHeader className="workspace-card__header">
            <h3>Sign in or create your creator account</h3>
            <p className="muted">An account is required to create channels and issue OBS settings.</p>
          </CardHeader>
          <div className="workspace-card__actions">
            <Button onClick={() => void signIn("/creator/getting-started")}>Sign in</Button>
            <Button variant="secondary" onClick={() => void signUp("/creator/getting-started")}>
              Create account
            </Button>
          </div>
        </Card>
      ) : null}

      <div className="step-grid">
        <Card className="workspace-card step-card" aria-labelledby="creator-step-1">
          <CardHeader className="workspace-card__header">
            <div className="step-card__status">
              <h3 id="creator-step-1">{hasChannel ? "1) Pick a channel" : "1) Create your first channel"}</h3>
              <Badge tone={step1Done ? "success" : "info"}>{step1Done ? "Complete" : "Pending"}</Badge>
            </div>
            <p className="muted">
              {hasChannel
                ? "Pick the channel for this checklist."
                : "Create the public channel viewers will watch."}
            </p>
          </CardHeader>
          <CardBody className="workspace-card__header">
            {needsFirstChannel ? (
              <div className="creator-live__section">
                <label className="input-stack" htmlFor="creator-channel-title">
                  <span>Channel name</span>
                  <input
                    id="creator-channel-title"
                    value={channelForm.title}
                    onChange={(event) => {
                      const value = event.currentTarget.value;
                      setChannelForm((prev) => ({ ...prev, title: value }));
                      setCreateChannelError(undefined);
                      setCreateChannelSuccess(undefined);
                    }}
                    placeholder="Your channel name"
                  />
                </label>
                <div className="creator-live__split">
                  <label className="input-stack" htmlFor="creator-channel-category">
                    <span>Primary category</span>
                    <input
                    id="creator-channel-category"
                    value={channelForm.category}
                    onChange={(event) => {
                      const value = event.currentTarget.value;
                      setChannelForm((prev) => ({ ...prev, category: value }));
                    }}
                    placeholder="Gaming, Music, Just Chatting..."
                  />
                  </label>
                  <label className="input-stack" htmlFor="creator-channel-tags">
                    <span>Tags</span>
                    <input
                    id="creator-channel-tags"
                    value={channelForm.tags}
                    onChange={(event) => {
                      const value = event.currentTarget.value;
                      setChannelForm((prev) => ({ ...prev, tags: value }));
                    }}
                    placeholder="retro, speedrun, co-op"
                  />
                  </label>
                </div>
                <div className="workspace-card__actions">
                  <Button
                    onClick={() => {
                      void handleCreateChannel();
                    }}
                    disabled={creatingChannel || !channelForm.title.trim()}
                  >
                    {creatingChannel ? "Creating channel..." : "Create channel"}
                  </Button>
                </div>
                <p className="muted">
                  Creator tools unlock after the first channel is created.
                </p>
                {createChannelError ? <p className="error">{createChannelError}</p> : null}
                {createChannelSuccess ? <p className="success">{createChannelSuccess}</p> : null}
              </div>
            ) : channels.length > 1 ? (
              <label className="input-stack" htmlFor="channel-select">
                <span>Channel</span>
                <select
                  id="channel-select"
                  value={selectedChannel?.id ?? ""}
                  onChange={(event) => setSelectedChannelId(event.currentTarget.value)}
                >
                  {channels.map((channel) => (
                    <option key={channel.id} value={channel.id}>
                      {channel.title}
                    </option>
                  ))}
                </select>
              </label>
            ) : channels.length === 1 ? (
              <p>
                Selected channel: <strong>{channels[0].title}</strong>
              </p>
            ) : (
              <p className="muted">Sign in to create or manage a channel.</p>
            )}
            {hasChannel ? (
              <div className="workspace-card__actions">
                <Link href="/creator" className={buttonClassName("secondary")}>
                  Open studio
                </Link>
              </div>
            ) : null}
            {createChannelSuccess ? <p className="success">{createChannelSuccess}</p> : null}
            {channelsLoading && <p className="muted">Loading channels...</p>}
            {channelsError && <p className="error">{channelsError}</p>}
          </CardBody>
        </Card>

        <Card className="workspace-card step-card" aria-labelledby="creator-step-2">
          <CardHeader className="workspace-card__header">
            <div className="step-card__status">
              <h3 id="creator-step-2">2) Copy your stream settings</h3>
              <Badge tone={hasChannel ? "info" : "neutral"}>{hasChannel ? "Next step" : "Choose a channel first"}</Badge>
            </div>
            <p className="muted">Copy the OBS server URL and stream key.</p>
          </CardHeader>
          <CardBody className="workspace-card__header">
            {hasChannel ? (
              <div className="workspace-card__actions">
                <Link href={liveSetupLink} className={buttonClassName()}>
                  Open live setup
                </Link>
              </div>
            ) : (
              <p className="muted">Create a channel to unlock OBS settings.</p>
            )}
            <label className="inline-check">
              <input
                type="checkbox"
                checked={manualChecks.obsConfigured}
                onChange={(event) => {
                  const checked = event.currentTarget.checked;
                  setManualChecks((prev) => ({ ...prev, obsConfigured: checked }));
                }}
                disabled={!hasChannel}
              />
              <span>I copied my OBS settings.</span>
            </label>
          </CardBody>
        </Card>

        <Card className="workspace-card step-card" aria-labelledby="creator-step-3">
          <CardHeader className="workspace-card__header">
            <div className="step-card__status">
              <h3 id="creator-step-3">3) Go live and share the channel</h3>
              <Badge tone={liveStatusTone}>{liveStatusLabel}</Badge>
            </div>
            <p className="muted">Check whether OBS is sending a live signal.</p>
          </CardHeader>
          <CardBody className="workspace-card__header">
            {hasChannel ? (
              <div className="workspace-card__actions">
                <Button variant="secondary" onClick={() => void refreshLiveStatus()} disabled={!selectedChannel || liveLoading}>
                  Check live status
                </Button>
                <span className="muted">Current status: {step3Done ? "Live" : "Offline"}</span>
              </div>
            ) : (
              <p className="muted">Create a channel to check its signal.</p>
            )}
            {liveLoading && <p className="muted">Checking live status...</p>}
            {liveError && <p className="error">{liveError}</p>}
          </CardBody>
        </Card>

        <Card className="workspace-card step-card" aria-labelledby="creator-step-4">
          <CardHeader className="workspace-card__header">
            <div className="step-card__status">
              <h3 id="creator-step-4">4) Share viewer link</h3>
              <Badge tone={step4Done ? "success" : "neutral"}>{step4Done ? "Complete" : "Manual confirmation"}</Badge>
            </div>
            <p className="muted">Copy the public channel URL.</p>
          </CardHeader>
          <CardBody className="workspace-card__header">
            {hasChannel ? (
              <div className="workspace-card__actions">
                <Button onClick={() => void handleCopyViewerLink()} disabled={!selectedChannel}>
                  Copy viewer link
                </Button>
                <Link href={viewerLink} className={buttonClassName("secondary")}>
                  Preview viewer page
                </Link>
              </div>
            ) : (
              <p className="muted">Create a channel to get its viewer link.</p>
            )}
            {copyMessage && <p className="muted">{copyMessage}</p>}
            <label className="inline-check">
              <input
                type="checkbox"
                checked={manualChecks.viewerLinkShared}
                onChange={(event) => {
                  const checked = event.currentTarget.checked;
                  setManualChecks((prev) => ({ ...prev, viewerLinkShared: checked }));
                }}
                disabled={!hasChannel}
              />
              <span>I shared my viewer link.</span>
            </label>
          </CardBody>
        </Card>

        <Card className="workspace-card step-card" aria-labelledby="creator-step-5">
          <CardHeader className="workspace-card__header">
            <div className="step-card__status">
              <h3 id="creator-step-5">5) Optional: Upload a test VOD</h3>
              <Badge tone={step5Done ? "success" : "neutral"}>{step5Done ? "Complete" : "Optional"}</Badge>
            </div>
            <p className="muted">Optional playback check after the stream.</p>
          </CardHeader>
          <CardBody className="workspace-card__header">
            {hasChannel ? (
              <div className="workspace-card__actions">
                <Link href={uploadsLink} className={buttonClassName("secondary")}>
                  Open uploads
                </Link>
              </div>
            ) : (
              <p className="muted">Create a channel to use uploads.</p>
            )}
            <label className="inline-check">
              <input
                type="checkbox"
                checked={manualChecks.vodUploaded}
                onChange={(event) => {
                  const checked = event.currentTarget.checked;
                  setManualChecks((prev) => ({ ...prev, vodUploaded: checked }));
                }}
                disabled={!hasChannel}
              />
              <span>I uploaded a test VOD.</span>
            </label>
          </CardBody>
        </Card>
      </div>
    </div>
  );
}
