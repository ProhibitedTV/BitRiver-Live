"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useState } from "react";
import { Badge } from "../../../components/ui/Badge";
import { Button, buttonClassName } from "../../../components/ui/Button";
import { Card, CardBody, CardHeader } from "../../../components/ui/Card";
import { useAuth } from "../../../hooks/useAuth";
import {
  ManagedChannel,
  ViewerApiError,
  fetchChannelPlayback,
  fetchManagedChannels,
  fetchProfile,
} from "../../../lib/viewer-api";

type ManualChecks = {
  ingestCopied: boolean;
  sharedViewerLink: boolean;
};

const MANUAL_CHECKS_STORAGE_KEY = "creator-getting-started-manual-checks";

function loadStoredChecks(): ManualChecks {
  if (typeof window === "undefined") {
    return { ingestCopied: false, sharedViewerLink: false };
  }
  try {
    const raw = window.localStorage.getItem(MANUAL_CHECKS_STORAGE_KEY);
    if (!raw) {
      return { ingestCopied: false, sharedViewerLink: false };
    }
    const parsed = JSON.parse(raw) as Partial<ManualChecks>;
    return {
      ingestCopied: Boolean(parsed.ingestCopied),
      sharedViewerLink: Boolean(parsed.sharedViewerLink),
    };
  } catch {
    return { ingestCopied: false, sharedViewerLink: false };
  }
}

export default function CreatorGettingStartedPage() {
  const { user, loading: authLoading, signIn } = useAuth();
  const [profileExists, setProfileExists] = useState(false);
  const [profileLoading, setProfileLoading] = useState(false);
  const [channels, setChannels] = useState<ManagedChannel[]>([]);
  const [channelsLoading, setChannelsLoading] = useState(false);
  const [channelsError, setChannelsError] = useState<string | undefined>();
  const [selectedChannelId, setSelectedChannelId] = useState<string>("");
  const [isLive, setIsLive] = useState(false);
  const [liveLoading, setLiveLoading] = useState(false);
  const [liveError, setLiveError] = useState<string | undefined>();
  const [copyMessage, setCopyMessage] = useState<string | undefined>();
  const [manualChecks, setManualChecks] = useState<ManualChecks>({ ingestCopied: false, sharedViewerLink: false });

  useEffect(() => {
    setManualChecks(loadStoredChecks());
  }, []);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    window.localStorage.setItem(MANUAL_CHECKS_STORAGE_KEY, JSON.stringify(manualChecks));
  }, [manualChecks]);

  useEffect(() => {
    if (!user) {
      setProfileExists(false);
      return;
    }

    let cancelled = false;
    const loadProfile = async () => {
      setProfileLoading(true);
      try {
        await fetchProfile(user.id);
        if (!cancelled) {
          setProfileExists(true);
        }
      } catch (err) {
        if (!cancelled) {
          if (err instanceof ViewerApiError && err.status === 404) {
            setProfileExists(false);
          } else {
            setProfileExists(false);
          }
        }
      } finally {
        if (!cancelled) {
          setProfileLoading(false);
        }
      }
    };

    void loadProfile();
    return () => {
      cancelled = true;
    };
  }, [user]);

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
    const id = window.setInterval(() => {
      void refreshLiveStatus();
    }, 10000);
    return () => {
      window.clearInterval(id);
    };
  }, [refreshLiveStatus, selectedChannel?.id]);

  const ingestInfo = useMemo(() => {
    if (!selectedChannel) {
      return undefined;
    }
    const endpoint = selectedChannel.ingestEndpoints?.[0] ?? "";
    const streamKey = selectedChannel.streamKey;
    return {
      endpoint,
      streamKey,
      combined: `${endpoint}${endpoint && streamKey ? "\n" : ""}${streamKey}`,
    };
  }, [selectedChannel]);

  const handleCopyStreamInfo = useCallback(async () => {
    if (!ingestInfo?.combined) {
      return;
    }
    try {
      await navigator.clipboard.writeText(ingestInfo.combined);
      setCopyMessage("Copied ingest info");
    } catch {
      setCopyMessage("Copy failed. Copy manually from the fields below.");
    }
  }, [ingestInfo?.combined]);

  const viewerLink = selectedChannel ? `/channels/${selectedChannel.id}` : "/browse";

  const step1Done = Boolean(user && profileExists);
  const step2Done = channels.length > 0;
  const step3Done = manualChecks.ingestCopied;
  const step4Done = isLive;
  const step5Done = manualChecks.sharedViewerLink;

  return (
    <div className="container" style={{ paddingTop: "2rem", paddingBottom: "4rem" }}>
      <Card>
        <CardHeader>
          <h1>Getting Started</h1>
          <p className="muted">Set up your creator workflow once, then go live faster next time.</p>
        </CardHeader>
        <CardBody style={{ gap: "1rem" }}>
          <Card>
            <h2 style={{ marginBottom: "0.5rem" }}>1) Create your profile (if required)</h2>
            <Badge tone={step1Done ? "success" : "info"}>{step1Done ? "Complete" : "Pending"}</Badge>
            <p className="muted">Create your profile so viewers see your public identity and branding.</p>
            {!user && !authLoading ? (
              <Button onClick={() => void signIn("/profile")}>Create profile</Button>
            ) : (
              <Link href="/profile" className={buttonClassName(step1Done ? "secondary" : "primary")}>Create profile</Link>
            )}
            {profileLoading && <p className="muted">Checking profile…</p>}
          </Card>

          <Card>
            <h2 style={{ marginBottom: "0.5rem" }}>2) Create or select a channel</h2>
            <Badge tone={step2Done ? "success" : "info"}>{step2Done ? "Complete" : "Pending"}</Badge>
            <p className="muted">Choose the channel you want to stream from.</p>
            {channels.length > 0 ? (
              <>
                <label htmlFor="channel-select">Managed channel</label>
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
                <Link href={`/creator/live/${selectedChannel?.id ?? channels[0].id}`} className={buttonClassName("secondary")}>
                  Open channel dashboard
                </Link>
              </>
            ) : (
              <Link href="/creator" className={buttonClassName("primary")}>Create channel</Link>
            )}
            {channelsLoading && <p className="muted">Loading channels…</p>}
            {channelsError && <p className="error">{channelsError}</p>}
          </Card>

          <Card>
            <h2 style={{ marginBottom: "0.5rem" }}>3) Copy stream ingest info (RTMP/SRT endpoint + stream key)</h2>
            <Badge tone={step3Done ? "success" : "neutral"}>{step3Done ? "Complete" : "Manual confirmation"}</Badge>
            <p className="muted">Copy your primary ingest endpoint and stream key into OBS (or your encoder).</p>
            <Button onClick={() => void handleCopyStreamInfo()} disabled={!ingestInfo?.combined}>
              Copy stream key
            </Button>
            {ingestInfo?.endpoint && <code>{ingestInfo.endpoint}</code>}
            {ingestInfo?.streamKey && <code>{ingestInfo.streamKey}</code>}
            {copyMessage && <p className="muted">{copyMessage}</p>}
            {/* TODO: add backend ingest handshake signal so this step can auto-complete without manual confirmation. */}
            <label style={{ display: "flex", gap: "0.5rem", alignItems: "center" }}>
              <input
                type="checkbox"
                checked={manualChecks.ingestCopied}
                onChange={(event) =>
                  setManualChecks((prev) => ({
                    ...prev,
                    ingestCopied: event.currentTarget.checked,
                  }))
                }
              />
              I copied the stream ingest info.
            </label>
          </Card>

          <Card>
            <h2 style={{ marginBottom: "0.5rem" }}>4) Confirm a test stream is live</h2>
            <Badge tone={step4Done ? "success" : "info"}>{step4Done ? "Complete" : "Pending"}</Badge>
            <p className="muted">We poll your channel playback status every 10s and mark this done once live.</p>
            <Button variant="secondary" onClick={() => void refreshLiveStatus()} disabled={!selectedChannel || liveLoading}>
              Check live status
            </Button>
            {liveLoading && <p className="muted">Checking live status…</p>}
            {liveError && <p className="error">{liveError}</p>}
          </Card>

          <Card>
            <h2 style={{ marginBottom: "0.5rem" }}>5) Share your viewer link</h2>
            <Badge tone={step5Done ? "success" : "neutral"}>{step5Done ? "Complete" : "Manual confirmation"}</Badge>
            <p className="muted">Open your public channel page and share it with your first viewers.</p>
            <Link href={viewerLink} className={buttonClassName("primary")}>Open viewer</Link>
            {/* TODO: add backend share attribution/analytics event so this step can auto-complete from real signals. */}
            <label style={{ display: "flex", gap: "0.5rem", alignItems: "center" }}>
              <input
                type="checkbox"
                checked={manualChecks.sharedViewerLink}
                onChange={(event) =>
                  setManualChecks((prev) => ({
                    ...prev,
                    sharedViewerLink: event.currentTarget.checked,
                  }))
                }
              />
              I shared my viewer link.
            </label>
          </Card>
        </CardBody>
      </Card>
    </div>
  );
}
