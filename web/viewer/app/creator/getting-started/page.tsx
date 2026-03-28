"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useState } from "react";
import { Badge } from "../../../components/ui/Badge";
import { Button, buttonClassName } from "../../../components/ui/Button";
import { Card, CardBody, CardHeader } from "../../../components/ui/Card";
import { useAuth } from "../../../hooks/useAuth";
import { buildViewerPath, buildViewerUrl } from "../../../lib/viewer-links";
import { ManagedChannel, fetchChannelPlayback, fetchManagedChannels } from "../../../lib/viewer-api";

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
  const viewerLink = selectedChannel ? buildViewerPath(`/channels/${selectedChannel.id}`) : buildViewerPath("/browse");
  const hasChannel = Boolean(selectedChannel?.id);
  const liveStatusTone = isLive ? "success" : "info";
  const liveStatusLabel = isLive ? "Live" : "Waiting for stream";

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

  return (
    <div className="workspace-shell">
      <section className="workspace-hero">
        <div className="workspace-hero__copy">
          <span className="page-eyebrow">Creator setup</span>
          <h2>Get your first stream live</h2>
          <p className="muted">
            Pick a channel, open the live setup page, then share the viewer link once the preview is working.
          </p>
        </div>
        <div className="workspace-hero__actions">
          <Link href={liveSetupLink} className={buttonClassName(hasChannel ? undefined : "secondary")}>
            Open live setup
          </Link>
          <Link href={viewerLink} className={buttonClassName("secondary")}>
            Open viewer page
          </Link>
        </div>
      </section>

      {!user && !authLoading ? (
        <Card className="workspace-card">
          <CardHeader className="workspace-card__header">
            <h3>Sign in to continue</h3>
            <p className="muted">Sign in to choose a channel and start the guided live setup.</p>
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
              <h3 id="creator-step-1">1) Choose your channel</h3>
              <Badge tone={hasChannel ? "success" : "info"}>{hasChannel ? "Ready" : "Needed"}</Badge>
            </div>
            <p className="muted">Use one channel for your first test so your stream key and viewer link stay consistent.</p>
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
              <h3 id="creator-step-2">2) Copy your stream settings</h3>
              <Badge tone={hasChannel ? "info" : "neutral"}>{hasChannel ? "Next step" : "Choose a channel first"}</Badge>
            </div>
            <p className="muted">The live setup page gives you the OBS server URL, stream key, preview, and share link in one place.</p>
          </CardHeader>
          <CardBody className="workspace-card__header">
            <div className="workspace-card__actions">
              <Link href={liveSetupLink} className={buttonClassName()}>
                Open live setup
              </Link>
            </div>
          </CardBody>
        </Card>

        <Card className="workspace-card step-card" aria-labelledby="creator-step-3">
          <CardHeader className="workspace-card__header">
            <div className="step-card__status">
              <h3 id="creator-step-3">3) Go live and share the channel</h3>
              <Badge tone={liveStatusTone}>{liveStatusLabel}</Badge>
            </div>
            <p className="muted">Once OBS is connected, check the live status here and copy the same viewer link your audience will use.</p>
          </CardHeader>
          <CardBody className="workspace-card__header">
            <div className="workspace-card__actions">
              <Button variant="secondary" onClick={() => void refreshLiveStatus()} disabled={!selectedChannel || liveLoading}>
                Check live status
              </Button>
              <Button onClick={() => void handleCopyViewerLink()} disabled={!selectedChannel}>
                Copy viewer link
              </Button>
              <Link href={viewerLink} className={buttonClassName("secondary")}>
                Preview viewer page
              </Link>
            </div>
            <p className="muted">Current status: {isLive ? "Live" : "Offline"}</p>
            {copyMessage && <p className="muted">{copyMessage}</p>}
            {liveLoading && <p className="muted">Checking live status...</p>}
            {liveError && <p className="error">{liveError}</p>}
          </CardBody>
        </Card>
      </div>
    </div>
  );
}
