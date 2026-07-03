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
            <h2>Open your studio</h2>
            <p className="muted">Sign in to create channels, copy stream settings, and manage replays.</p>
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
    <section className="creator-overview-grid creator-overview-grid--simple" aria-label="Creator overview">
      <article className="creator-overview-card creator-overview-card--primary">
        <span className="page-eyebrow">{hasChannels ? "Go live" : "First stream"}</span>
        <h2>{hasChannels ? "Resume live setup" : "Create your first channel"}</h2>
        <p className="muted">{hasChannels ? "Open OBS settings, preview, and sharing." : "Create one public channel to unlock creator tools."}</p>
        <div className="creator-overview-card__actions">
          <Link href={hasChannels ? liveHref : "/creator/getting-started"} className={buttonClassName()}>
            {hasChannels ? "Open live setup" : "Start setup"}
          </Link>
          <Link href="/creator/getting-started" className={buttonClassName("secondary")}>
            Checklist
          </Link>
        </div>
      </article>

      <article className="creator-overview-card creator-overview-card--compact">
        <span className="page-eyebrow">Uploads</span>
        <h2>Replays</h2>
        <p className="muted">Register VODs and track processing.</p>
        <div className="creator-overview-card__actions">
          <Link href={uploadsHref} className={buttonClassName("secondary")}>
            {hasChannels ? "Open uploads" : "Unlock uploads"}
          </Link>
        </div>
      </article>

      <article className="creator-overview-card creator-overview-card--compact">
        <span className="page-eyebrow">Status</span>
        <h2>{hasChannels ? primaryChannel?.title ?? "Channel ready" : "No channel yet"}</h2>
        <p className="muted">
          {channelsLoading
            ? "Loading setup..."
            : hasChannels
              ? "Ready for live setup and playback."
              : "Create a channel to unlock OBS settings."}
        </p>
        {channelsError ? <p className="error">{channelsError}</p> : null}
      </article>
    </section>
  );
}
