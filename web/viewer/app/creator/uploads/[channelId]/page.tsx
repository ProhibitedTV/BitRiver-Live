"use client";

import Link from "next/link";
import { UploadManager } from "../../../../components/UploadManager";
import { useCreatorChannel } from "../../../../hooks/useCreatorChannel";

export default function CreatorUploadsPage() {
  const { playback, loading, error, channelId, reload } = useCreatorChannel();

  if (loading) {
    return <section className="surface">Loading channel…</section>;
  }

  if (error) {
    return (
      <section className="surface stack">
        <h2>Unable to load channel</h2>
        <p className="error">{error}</p>
        <button type="button" className="secondary-button" onClick={() => { void reload(false); }}>
          Try again
        </button>
      </section>
    );
  }

  if (!playback) {
    return (
      <section className="surface stack">
        <h2>Channel not available</h2>
        <p className="muted">We couldn&apos;t find channel details for this dashboard.</p>
      </section>
    );
  }

  return (
    <div className="stack" style={{ gap: "1.5rem" }}>
      <header className="stack">
        <h2>Manage uploads for {playback.channel.title}</h2>
        <p className="muted">Register VODs after streams wrap and monitor processing progress.</p>
        <Link href={`/channels/${channelId}`} className="secondary-button" style={{ alignSelf: "flex-start" }}>
          View public channel
        </Link>
      </header>
      <UploadManager channelId={channelId} ownerId={playback.channel.ownerId} />
    </div>
  );
}
