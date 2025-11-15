import Link from "next/link";
import type { DirectoryChannel } from "../lib/viewer-api";

interface FollowingRailProps {
  channels: DirectoryChannel[];
  loading?: boolean;
}

export function FollowingRail({ channels, loading = false }: FollowingRailProps) {
  return (
    <section className="following-rail surface" id="following">
      <header className="following-rail__header">
        <div className="stack">
          <span className="following-rail__eyebrow muted">Following</span>
          <h3>Catch up with your creators</h3>
        </div>
        {channels.length > 0 && !loading && <span className="muted">{channels.length} online</span>}
      </header>
      {loading ? (
        <p className="muted">Checking who is live…</p>
      ) : channels.length === 0 ? (
        <p className="muted">
          You&rsquo;re not following any channels yet. Follow a broadcaster to see their stream here the moment they go live.
        </p>
      ) : (
        <div className="following-rail__scroller" role="list">
          {channels.map((entry) => {
            const avatar = entry.profile.avatarUrl ?? entry.profile.bannerUrl;
            const ownerInitial = entry.owner.displayName.charAt(0).toUpperCase() || "B";
            return (
              <Link key={entry.channel.id} href={`/channels/${entry.channel.id}`} className="following-card" role="listitem">
                <div className="following-card__avatar">
                  {avatar ? (
                    <img src={avatar} alt={entry.owner.displayName} />
                  ) : (
                    <span aria-hidden="true">{ownerInitial}</span>
                  )}
                  {entry.live && <span className="following-card__status" aria-label="Live" />}
                </div>
                <div className="following-card__meta">
                  <strong>{entry.owner.displayName}</strong>
                  <span className="muted">
                    {entry.channel.category ?? "Variety"}
                    {entry.channel.category && entry.channel.liveState ? " • " : ""}
                    {entry.channel.liveState}
                  </span>
                </div>
              </Link>
            );
          })}
        </div>
      )}
    </section>
  );
}
