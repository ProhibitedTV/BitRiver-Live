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
    <section className="grid directory-grid">
      {channels.map((entry) => {
        const createdAt = new Date(entry.channel.createdAt).toLocaleDateString();
        const previewImage = entry.profile.bannerUrl ?? entry.profile.avatarUrl;
        const followerLabel = `${entry.followerCount.toLocaleString()} follower${entry.followerCount === 1 ? "" : "s"}`;

        return (
          <article key={entry.channel.id} className="directory-card">
            <Link href={`/channels/${entry.channel.id}`} className="directory-card__link">
              <div className="directory-card__preview">
                {previewImage ? (
                  <img
                    src={previewImage}
                    alt={`${entry.owner.displayName} channel artwork`}
                    className="directory-card__media"
                  />
                ) : (
                  <div className="directory-card__preview-fallback" aria-hidden="true" />
                )}
                <div className="overlay overlay--top overlay--scrim">
                  {entry.live && <span className="badge badge--live">Live</span>}
                  {entry.live ? (
                    <span className="overlay__meta">{`${entry.followerCount.toLocaleString()} viewers`}</span>
                  ) : (
                    <span className="overlay__meta overlay__meta--muted">Offline</span>
                  )}
                </div>
              </div>
              <div className="directory-card__content">
                <div className="directory-card__header">
                  <h3 className="directory-card__title">{entry.channel.title}</h3>
                  <span className="directory-card__subtitle muted">
                    {entry.owner.displayName} &middot; {createdAt}
                  </span>
                </div>
                {entry.profile.bio && <p className="directory-card__description muted">{entry.profile.bio}</p>}
                <div className="tag-list">
                  {entry.channel.category && <span className="tag">{entry.channel.category}</span>}
                  {entry.channel.tags.slice(0, 3).map((tag) => (
                    <span key={tag} className="tag">
                      #{tag}
                    </span>
                  ))}
                </div>
              </div>
            </Link>
            <footer className="directory-card__footer">
              <span className="muted">{followerLabel}</span>
              <Link className="secondary-button" href={`/channels/${entry.channel.id}`}>
                View channel
              </Link>
            </footer>
          </article>
        );
      })}
    </section>
  );
}
