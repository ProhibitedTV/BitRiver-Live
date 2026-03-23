"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useState } from "react";
import { Badge } from "../../../components/ui/Badge";
import { Button, buttonClassName } from "../../../components/ui/Button";
import { Card, CardBody, CardHeader } from "../../../components/ui/Card";
import { useAuth } from "../../../hooks/useAuth";
import { buildViewerPath, buildViewerUrl } from "../../../lib/viewer-links";
import { ManagedChannel, fetchChannelPlayback, fetchManagedChannels } from "../../../lib/viewer-api";

type ManualChecks = {
  obsConfigured: boolean;
  viewerLinkShared: boolean;
  vodUploaded: boolean;
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

export default function CreatorGettingStartedPage() {
  const { user, loading: authLoading, signIn } = useAuth();
  const [channels, setChannels] = useState<ManagedChannel[]>([]);
  const [channelsLoading, setChannelsLoading] = useState(false);
  const [channelsError, setChannelsError] = useState<string | undefined>();
  const [selectedChannelId, setSelectedChannelId] = useState<string>("");
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
      setIsLive(Boolean(playback.live));
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

  const hasChannel = Boolean(selectedChannel?.id);
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
            This checklist moves from channel setup to OBS to public sharing so you always know the next step.
          </p>
        </div>
        <div className="workspace-hero__actions">
          <Link href={viewerLink} className={buttonClassName("secondary")}>
            Open public preview
          </Link>
          <Link href="/creator" className={buttonClassName("secondary")}>
            Open studio overview
          </Link>
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
            <h3>Sign in to continue</h3>
            <p className="muted">Sign in to pick your channel and finish onboarding.</p>
          </CardHeader>
          <div className="workspace-card__actions">
            <Button onClick={() => void signIn("/creator/getting-started")}>Sign in</Button>
          </div>
        </Card>
      ) : null}

      <div className="step-grid">
        <Card className="workspace-card step-card" aria-labelledby="creator-step-1">
          <CardHeader className="workspace-card__header">
            <div className="step-card__status">
              <h3 id="creator-step-1">1) Pick a channel</h3>
              <Badge tone={step1Done ? "success" : "info"}>{step1Done ? "Complete" : "Pending"}</Badge>
            </div>
            <p className="muted">Select the channel you want to use for onboarding before you copy any streaming settings.</p>
          </CardHeader>
          <CardBody className="workspace-card__header">
            {channels.length > 1 ? (
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
              <p className="muted">No channels found yet.</p>
            )}
            <div className="workspace-card__actions">
              <Link href="/creator" className={buttonClassName(hasChannel ? "secondary" : "primary")}>
                Manage channels
              </Link>
            </div>
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
            <div className="workspace-card__actions">
              <Link href={liveSetupLink} className={buttonClassName()}>
                Open live setup
              </Link>
            </div>
            <label className="inline-check">
              <input
                type="checkbox"
                checked={manualChecks.obsConfigured}
                onChange={(event) => {
                  const checked = event.currentTarget.checked;
                  setManualChecks((prev) => ({ ...prev, obsConfigured: checked }));
                }}
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
            <div className="workspace-card__actions">
              <Button variant="secondary" onClick={() => void refreshLiveStatus()} disabled={!selectedChannel || liveLoading}>
                Check live status
              </Button>
              <span className="muted">Current status: {step3Done ? "Live" : "Offline"}</span>
            </div>
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
            <div className="workspace-card__actions">
              <Button onClick={() => void handleCopyViewerLink()} disabled={!selectedChannel}>
                Copy viewer link
              </Button>
              <Link href={viewerLink} className={buttonClassName("secondary")}>
                Preview viewer page
              </Link>
            </div>
            {copyMessage && <p className="muted">{copyMessage}</p>}
            <label className="inline-check">
              <input
                type="checkbox"
                checked={manualChecks.viewerLinkShared}
                onChange={(event) => {
                  const checked = event.currentTarget.checked;
                  setManualChecks((prev) => ({ ...prev, viewerLinkShared: checked }));
                }}
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
            <div className="workspace-card__actions">
              <Link href={uploadsLink} className={buttonClassName("secondary")}>
                Open uploads
              </Link>
            </div>
            <label className="inline-check">
              <input
                type="checkbox"
                checked={manualChecks.vodUploaded}
                onChange={(event) => {
                  const checked = event.currentTarget.checked;
                  setManualChecks((prev) => ({ ...prev, vodUploaded: checked }));
                }}
              />
              <span>I uploaded a test VOD.</span>
            </label>
          </CardBody>
        </Card>
      </div>
    </div>
  );
}
