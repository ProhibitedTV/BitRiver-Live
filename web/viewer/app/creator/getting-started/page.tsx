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
  const [manualChecks, setManualChecks] = useState<ManualChecks>({
    obsConfigured: false,
    viewerLinkShared: false,
    vodUploaded: false,
  });

  useEffect(() => {
    setManualChecks(loadStoredChecks());
  }, []);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    window.localStorage.setItem(MANUAL_CHECKS_STORAGE_KEY, JSON.stringify(manualChecks));
  }, [manualChecks]);

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
  }, [selectedChannel?.id]);

  useEffect(() => {
    void refreshLiveStatus();
  }, [refreshLiveStatus]);

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

  return (
    <div className="workspace-shell">
      <section className="workspace-hero">
        <div className="workspace-hero__copy">
          <span className="page-eyebrow">Creator onboarding</span>
          <h2>Get your first stream ready without guesswork</h2>
          <p className="muted">
            This checklist moves from first-channel setup to OBS to public sharing so you always know the next move.
          </p>
        </div>
        <div className="workspace-hero__actions">
          {hasChannel ? (
            <>
              <Link href={viewerLink} className={buttonClassName("secondary")}>
                Open public preview
              </Link>
              <Link href="/creator" className={buttonClassName("secondary")}>
                Open studio overview
              </Link>
            </>
          ) : (
            <>
              <Link href="/browse" className={buttonClassName("secondary")}>
                Browse live channels
              </Link>
              <Link href="#creator-step-1" className={buttonClassName("secondary")}>
                Create first channel
              </Link>
            </>
          )}
        </div>
        <div className="workspace-summary-grid">
          <article className="summary-card">
            <span className="summary-card__label">Progress</span>
            <strong className="summary-card__value">{completedSteps}/5</strong>
            <p className="muted">Checklist steps complete</p>
          </article>
          <article className="summary-card">
            <span className="summary-card__label">Selected channel</span>
            <strong className="summary-card__value">{selectedChannel?.title ?? "None"}</strong>
            <p className="muted">Switch channels any time before you go live.</p>
          </article>
          <article className="summary-card">
            <span className="summary-card__label">Live signal</span>
            <strong className="summary-card__value">{step3Done ? "Live" : "Offline"}</strong>
            <p className="muted">This state updates automatically from playback.</p>
          </article>
        </div>
      </section>

      {!user && !authLoading ? (
        <Card className="workspace-card">
          <CardHeader className="workspace-card__header">
            <h3>Sign in or create your creator account</h3>
            <p className="muted">You need an account before BitRiver can create your first channel and issue OBS settings.</p>
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
                ? "Select the channel you want to use for onboarding before you copy any streaming settings."
                : "Create the public channel your audience will watch, then BitRiver will unlock the rest of the go-live flow."}
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
                  We’ll upgrade this account for creator tools automatically once your first channel is created.
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
                  Open studio overview
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
              <h3 id="creator-step-2">2) Copy OBS settings</h3>
              <Badge tone={step2Done ? "success" : "neutral"}>{step2Done ? "Complete" : "Manual confirmation"}</Badge>
            </div>
            <p className="muted">Open the guided live setup to copy the ingest endpoint and stream key into OBS.</p>
          </CardHeader>
          <CardBody className="workspace-card__header">
            {hasChannel ? (
              <div className="workspace-card__actions">
                <Link href={liveSetupLink} className={buttonClassName()}>
                  Open live setup
                </Link>
              </div>
            ) : (
              <p className="muted">Create your first channel in step 1 and this page will unlock your OBS settings immediately.</p>
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
              <h3 id="creator-step-3">3) Go live</h3>
              <Badge tone={step3Done ? "success" : "info"}>{step3Done ? "Complete" : "Pending"}</Badge>
            </div>
            <p className="muted">We read the public playback signal so you can confirm the platform is receiving your stream.</p>
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
              <p className="muted">Once your first channel exists, BitRiver will watch for its public live signal here.</p>
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
            <p className="muted">Copy the public channel page that viewers will use once your stream is ready.</p>
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
              <p className="muted">Your public viewer link appears here as soon as step 1 is complete.</p>
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
            <p className="muted">Use one short recording to confirm that uploads and playback behave the way you expect.</p>
          </CardHeader>
          <CardBody className="workspace-card__header">
            {hasChannel ? (
              <div className="workspace-card__actions">
                <Link href={uploadsLink} className={buttonClassName("secondary")}>
                  Open uploads
                </Link>
              </div>
            ) : (
              <p className="muted">Once your channel is live, uploads are the fastest way to verify VOD playback next.</p>
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
