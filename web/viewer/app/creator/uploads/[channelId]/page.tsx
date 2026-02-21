"use client";

import Link from "next/link";
import { UploadManager } from "../../../../components/UploadManager";
import { Button, buttonClassName } from "../../../../components/ui/Button";
import { Card, CardHeader } from "../../../../components/ui/Card";
import { EmptyState } from "../../../../components/ui/EmptyState";
import { InlineAlert } from "../../../../components/ui/InlineAlert";
import { useCreatorChannel } from "../../../../hooks/useCreatorChannel";

export default function CreatorUploadsPage() {
  const { playback, loading, error, channelId, reload } = useCreatorChannel();

  if (loading) {
    return <Card>Loading channel…</Card>;
  }

  if (error) {
    return (
      <Card>
        <h2>Unable to load channel</h2>
        <InlineAlert>{error}</InlineAlert>
        <Button variant="secondary" onClick={() => { void reload(false); }}>
          Try again
        </Button>
      </Card>
    );
  }

  if (!playback) {
    return (
      <EmptyState>
        <h2>Channel not available</h2>
        <p className="muted">We couldn&apos;t find channel details for this dashboard.</p>
      </EmptyState>
    );
  }

  return (
    <div className="stack" style={{ gap: "1.5rem" }}>
      <CardHeader>
        <h2>Manage uploads for {playback.channel.title}</h2>
        <p className="muted">Register VODs after streams wrap and monitor processing progress.</p>
        <Link href={`/channels/${channelId}`} className={buttonClassName("secondary")} style={{ alignSelf: "flex-start" }}>
          View public channel
        </Link>
      </CardHeader>
      <UploadManager channelId={channelId} ownerId={playback.channel.ownerId} />
    </div>
  );
}
