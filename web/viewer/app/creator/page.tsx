"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useState } from "react";
import { Button, buttonClassName } from "../../components/ui/Button";
import { Card, CardBody, CardHeader } from "../../components/ui/Card";
import { useAuth } from "../../hooks/useAuth";
import { ManagedChannel, fetchManagedChannels } from "../../lib/viewer-api";

export default function CreatorIndexPage() {
  const { user, loading: authLoading, signIn, signUp } = useAuth();
  const [channels, setChannels] = useState<ManagedChannel[]>([]);
  const [channelsLoading, setChannelsLoading] = useState(false);
  const [channelsError, setChannelsError] = useState<string | undefined>();

  const loadChannels = useCallback(async () => {
    if (!user) {
      setChannels([]);
      setChannelsError(undefined);
      return;
    }

    setChannelsLoading(true);
    setChannelsError(undefined);
    try {
      const response = await fetchManagedChannels();
      setChannels(response);
    } catch (err) {
      setChannels([]);
      setChannelsError(err instanceof Error ? err.message : "Unable to load channels");
    } finally {
      setChannelsLoading(false);
    }
  }, [user]);

  useEffect(() => {
    void loadChannels();
  }, [loadChannels]);

  const primaryChannel = useMemo(() => channels[0], [channels]);
  const liveHref = primaryChannel ? `/creator/live/${primaryChannel.id}` : "/creator/getting-started";
  const uploadsHref = primaryChannel ? `/creator/uploads/${primaryChannel.id}` : "/creator/getting-started";
  const hasChannels = channels.length > 0;

  if (!user && !authLoading) {
    return (
      <section className="creator-overview-grid" aria-label="Creator overview">
        <Card className="creator-overview-card">
          <CardHeader className="workspace-card__header">
            <span className="page-eyebrow">Creator access</span>
            <h2>Sign in to open your studio</h2>
            <p className="muted">
              BitRiver will guide you from account creation to channel setup to OBS and your public viewer link.
            </p>
          </CardHeader>
          <CardBody className="workspace-card__actions">
            <Button onClick={() => void signIn("/creator")}>Sign in</Button>
            <Button variant="secondary" onClick={() => void signUp("/creator/getting-started")}>
              Create account
            </Button>
          </CardBody>
        </Card>
      </section>
    );
  }

  return (
    <section className="creator-overview-grid" aria-label="Creator overview">
      <article className="creator-overview-card">
        <span className="page-eyebrow">{hasChannels ? "Go live" : "First stream"}</span>
        <h2>{hasChannels ? "Jump back into your live workflow" : "Create your first channel"}</h2>
        <p className="muted">
          {hasChannels
            ? "Pick up where you left off with OBS settings, preview checks, and a share-ready viewer link."
            : "Start with one public channel. BitRiver will upgrade this account for creator tools automatically after creation."}
        </p>
        <div className="creator-overview-card__actions">
          <Link href={hasChannels ? liveHref : "/creator/getting-started"} className={buttonClassName()}>
            {hasChannels ? "Open go-live dashboard" : "Open first-channel setup"}
          </Link>
        </div>
      </article>

      <article className="creator-overview-card">
        <span className="page-eyebrow">Onboarding</span>
        <h2>Use the guided checklist</h2>
        <p className="muted">
          The checklist keeps channel setup, live confirmation, sharing, and VOD follow-up in one place so nothing gets skipped.
        </p>
        <div className="creator-overview-card__actions">
          <Link href="/creator/getting-started" className={buttonClassName("secondary")}>
            Open getting started
          </Link>
        </div>
      </article>

      <article className="creator-overview-card">
        <span className="page-eyebrow">Uploads</span>
        <h2>Keep replays visible</h2>
        <p className="muted">
          Register VODs, watch processing states, and jump back to the same public page your audience will use.
        </p>
        <div className="creator-overview-card__actions">
          <Link href={uploadsHref} className={buttonClassName("secondary")}>
            {hasChannels ? "Open uploads" : "Unlock with first channel"}
          </Link>
        </div>
      </article>

      <article className="creator-overview-card">
        <span className="page-eyebrow">Status</span>
        <h2>{hasChannels ? primaryChannel?.title ?? "Channel ready" : "No channel yet"}</h2>
        <p className="muted">
          {channelsLoading
            ? "Loading your creator setup..."
            : hasChannels
              ? "Your primary channel is ready for live setup and public playback."
              : "Create your first channel to unlock OBS settings, preview monitoring, and viewer sharing."}
        </p>
        {channelsError ? <p className="error">{channelsError}</p> : null}
      </article>
    </section>
  );
}
