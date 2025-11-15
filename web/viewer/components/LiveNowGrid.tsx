import Link from "next/link";
import type { DirectoryChannel } from "../lib/viewer-api";

interface LiveNowGridProps {
  channels: DirectoryChannel[];
  loading?: boolean;
}

export function LiveNowGrid({ channels, loading = false }: LiveNowGridProps) {
  if (loading) {
    return <div className="surface" aria-busy="true">Loading live channels…</div>;
  }

  if (channels.length === 0) {
    return (
      <div className="surface">
        <h3>Nobody is live right now</h3>
        <p className="muted">As soon as creators go live, their broadcasts will show up here.</p>
      </div>
    );
  }

  return (
    <div className="grid live-now-grid">
      {channels.map((entry) => {
        const previewImage = entry.profile.bannerUrl ?? entry.profile.avatarUrl;
        return (
          <Link key={entry.channel.id} className="live-card" href={`/channels/${entry.channel.id}`}>
            <div className="live-card__media">
              {previewImage ? (
                <img src={previewImage} alt={`${entry.owner.displayName} channel artwork`} />
              ) : (
                <div className="live-card__media-fallback" aria-hidden="true" />
              )}
              <div className="overlay overlay--top overlay--scrim">
                <span className="badge badge--live">Live</span>
                <span className="overlay__meta">{entry.channel.category ?? "Streaming"}</span>
              </div>
            </div>
            <div className="live-card__body">
              <h3>{entry.channel.title}</h3>
              <p className="muted">{entry.owner.displayName}</p>
            </div>
          </Link>
        );
      })}
    </div>
  );
}
