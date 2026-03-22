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

  return (
    <div className="container" style={{ paddingTop: "2rem", paddingBottom: "4rem" }}>
      <Card>
        <CardHeader>
          <h1>Creator getting started</h1>
          <p className="muted">Follow this checklist to set up your channel and run your first live stream.</p>
        </CardHeader>
        <CardBody style={{ gap: "1rem" }}>
          {!user && !authLoading ? (
            <Card>
              <h2 style={{ marginBottom: "0.5rem" }}>Sign in to continue</h2>
              <p className="muted">Sign in to pick your channel and finish onboarding.</p>
              <Button onClick={() => void signIn("/creator/getting-started")}>Sign in</Button>
            </Card>
          ) : null}

          <Card>
            <h2 style={{ marginBottom: "0.5rem" }}>1) Pick a channel</h2>
            <Badge tone={step1Done ? "success" : "info"}>{step1Done ? "Complete" : "Pending"}</Badge>
            <p className="muted">Select the channel you want to use for onboarding.</p>
            {channels.length > 1 ? (
              <label htmlFor="channel-select">
                Channel
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
            <Link href="/creator" className={buttonClassName(hasChannel ? "secondary" : "primary")}>Manage channels</Link>
            {channelsLoading && <p className="muted">Loading channels…</p>}
            {channelsError && <p className="error">{channelsError}</p>}
          </Card>

          <Card>
            <h2 style={{ marginBottom: "0.5rem" }}>2) Copy OBS settings</h2>
            <Badge tone={step2Done ? "success" : "neutral"}>{step2Done ? "Complete" : "Manual confirmation"}</Badge>
            <p className="muted">Open your live control room to copy stream key and ingest endpoint into OBS.</p>
            <Link href={liveSetupLink} className={buttonClassName("primary")}>Open live setup</Link>
            <label style={{ display: "flex", gap: "0.5rem", alignItems: "center" }}>
              <input
                type="checkbox"
                checked={manualChecks.obsConfigured}
                onChange={(event) => {
                  const checked = event.currentTarget.checked;
                  setManualChecks((prev) => ({
                    ...prev,
                    obsConfigured: checked,
                  }));
                }}
              />
              I copied my OBS settings.
            </label>
          </Card>

          <Card>
            <h2 style={{ marginBottom: "0.5rem" }}>3) Go live</h2>
            <Badge tone={step3Done ? "success" : "info"}>{step3Done ? "Complete" : "Pending"}</Badge>
            <p className="muted">We detect this automatically from your channel’s live playback status.</p>
            <Button variant="secondary" onClick={() => void refreshLiveStatus()} disabled={!selectedChannel || liveLoading}>
              Check live status
            </Button>
            {liveLoading && <p className="muted">Checking live status…</p>}
            {liveError && <p className="error">{liveError}</p>}
            <p className="muted">Current status: {step3Done ? "Live" : "Offline"}</p>
          </Card>

          <Card>
            <h2 style={{ marginBottom: "0.5rem" }}>4) Share viewer link</h2>
            <Badge tone={step4Done ? "success" : "neutral"}>{step4Done ? "Complete" : "Manual confirmation"}</Badge>
            <p className="muted">Copy your public channel page and share it with viewers.</p>
            <Button onClick={() => void handleCopyViewerLink()} disabled={!selectedChannel}>
              Copy viewer link
            </Button>
            {copyMessage && <p className="muted">{copyMessage}</p>}
            <label style={{ display: "flex", gap: "0.5rem", alignItems: "center" }}>
              <input
                type="checkbox"
                checked={manualChecks.viewerLinkShared}
                onChange={(event) => {
                  const checked = event.currentTarget.checked;
                  setManualChecks((prev) => ({
                    ...prev,
                    viewerLinkShared: checked,
                  }));
                }}
              />
              I shared my viewer link.
            </label>
          </Card>

          <Card>
            <h2 style={{ marginBottom: "0.5rem" }}>5) Optional: Upload a test VOD</h2>
            <Badge tone={step5Done ? "success" : "neutral"}>{step5Done ? "Complete" : "Optional"}</Badge>
            <p className="muted">Verify uploads and playback by submitting one short VOD.</p>
            <Link href={uploadsLink} className={buttonClassName("secondary")}>Open uploads</Link>
            <label style={{ display: "flex", gap: "0.5rem", alignItems: "center" }}>
              <input
                type="checkbox"
                checked={manualChecks.vodUploaded}
                onChange={(event) => {
                  const checked = event.currentTarget.checked;
                  setManualChecks((prev) => ({
                    ...prev,
                    vodUploaded: checked,
                  }));
                }}
              />
              I uploaded a test VOD.
            </label>
          </Card>

          <p className="muted">Need help? You can reopen this checklist anytime from the creator dashboard.</p>
          <Link href={viewerLink} className={buttonClassName("secondary")}>Preview viewer page</Link>
        </CardBody>
      </Card>
    </div>
  );
}
