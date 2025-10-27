import Link from "next/link";
import type { DirectoryChannel } from "../lib/viewer-api";

export function DirectoryGrid({ channels }: { channels: DirectoryChannel[] }) {
  if (channels.length === 0) {
    return (
      <div className="surface stack">
        <h2>No channels yet</h2>
        <p className="muted">
          Creators haven’t gone live yet. Check back soon or invite your favourite broadcasters to join BitRiver Live.
        </p>
      </div>
    );
  }

  return (
    <section className="grid grid-columns-3">
      {channels.map((entry) => (
        <article key={entry.channel.id} className="card stack">
          <header className="stack">
            <Link href={`/channels/${entry.channel.id}`} className="stack">
              <div className="stack" style={{ gap: "0.35rem" }}>
                <h3>{entry.channel.title}</h3>
                <span className="muted">
                  {entry.owner.displayName} &middot; {new Date(entry.channel.createdAt).toLocaleDateString()}
                </span>
              </div>
              {entry.profile.avatarUrl && (
                <img
                  src={entry.profile.avatarUrl}
                  alt={`${entry.owner.displayName} avatar`}
                  style={{ width: "100%", maxHeight: "180px", objectFit: "cover" }}
                />
              )}
            </Link>
          </header>
          <div className="stack" style={{ gap: "0.5rem" }}>
            <div className="tag-list">
              {entry.live && <span className="badge">Live now</span>}
              {entry.channel.category && <span className="tag">{entry.channel.category}</span>}
              {entry.channel.tags.slice(0, 3).map((tag) => (
                <span key={tag} className="tag">
                  #{tag}
                </span>
              ))}
            </div>
            {entry.profile.bio && <p className="muted">{entry.profile.bio}</p>}
            <footer className="muted" style={{ display: "flex", justifyContent: "space-between" }}>
              <span>
                {entry.followerCount} follower{entry.followerCount === 1 ? "" : "s"}
              </span>
              <Link className="secondary-button" href={`/channels/${entry.channel.id}`}>
                View channel
              </Link>
            </footer>
          </div>
        </article>
      ))}
    </section>
  );
}
