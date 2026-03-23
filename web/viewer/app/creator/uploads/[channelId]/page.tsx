"use client";

import Link from "next/link";
import { UploadManager } from "../../../../components/UploadManager";
import { buttonClassName } from "../../../../components/ui/Button";
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
    <div className="workspace-shell">
      <section className="workspace-hero">
        <div className="workspace-hero__copy">
          <span className="page-eyebrow">Uploads</span>
          <h2>Manage uploads for {playback.channel.title}</h2>
          <p className="muted">
            Register VODs after a stream wraps, monitor processing, and jump back to the public channel when playback is ready.
          </p>
        </div>
        <div className="workspace-hero__actions">
          <Link href={`/creator/live/${channelId}`} className={buttonClassName("secondary")}>
            Open go live
          </Link>
          <Link href={`/channels/${channelId}`} className={buttonClassName("secondary")}>
            View public channel
          </Link>
        </div>
        <div className="workspace-summary-grid">
          <article className="summary-card">
            <span className="summary-card__label">Channel</span>
            <strong className="summary-card__value">{playback.channel.title}</strong>
            <p className="muted">Use this workspace to keep uploads tied to the right public page.</p>
          </article>
          <article className="summary-card">
            <span className="summary-card__label">Current live state</span>
            <strong className="summary-card__value">{playback.channel.liveState || "offline"}</strong>
            <p className="muted">Upload follow-up lives beside the live dashboard so creators can move between both.</p>
          </article>
          <article className="summary-card">
            <span className="summary-card__label">Next action</span>
            <strong className="summary-card__value">Review playback</strong>
            <p className="muted">Check for ready recordings and open the public page once media is available.</p>
          </article>
        </div>
      </section>

      <UploadManager channelId={channelId} ownerId={playback.channel.ownerId} />
    </div>
  );
}
