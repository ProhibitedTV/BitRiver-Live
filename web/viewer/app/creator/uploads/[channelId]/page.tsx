"use client";

import { ChannelStudioNav } from "../../../../components/ChannelStudioNav";
import { UploadManager } from "../../../../components/UploadManager";
import { Card } from "../../../../components/ui/Card";
import { EmptyState } from "../../../../components/ui/EmptyState";
import { InlineAlert } from "../../../../components/ui/InlineAlert";
import { useCreatorChannel } from "../../../../hooks/useCreatorChannel";

export default function CreatorUploadsPage() {
  const { playback, loading, error, channelId, reload } = useCreatorChannel();

  if (loading) {
    return <Card className="workspace-card">Loading channel...</Card>;
  }

  if (error) {
    return (
      <Card className="workspace-card">
        <h2>Unable to load channel</h2>
        <InlineAlert>{error}</InlineAlert>
        <div className="workspace-card__actions">
          <button
            type="button"
            className="secondary-button"
            onClick={() => {
              void reload(false);
            }}
          >
            Try again
          </button>
        </div>
      </Card>
    );
  }

  if (!playback) {
    return (
      <EmptyState className="workspace-card">
        <h2>Channel not available</h2>
        <p className="muted">We could not find channel details for this dashboard.</p>
      </EmptyState>
    );
  }

  return (
    <div className="channel-studio-workspace">
      <ChannelStudioNav
        channelId={channelId}
        channelTitle={playback.channel.title}
        liveState={playback.channel.liveState}
        activeTool="uploads"
        description="Register VODs after a stream wraps, monitor processing, and jump back to the same public channel when playback is ready."
      />

      <UploadManager channelId={channelId} ownerId={playback.channel.ownerId} />
    </div>
  );
}
