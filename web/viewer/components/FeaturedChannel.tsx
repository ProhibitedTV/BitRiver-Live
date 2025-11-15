import Link from "next/link";
import type { DirectoryChannel } from "../lib/viewer-api";

interface FeaturedChannelProps {
  channel?: DirectoryChannel;
  loading?: boolean;
}

export function FeaturedChannel({ channel, loading = false }: FeaturedChannelProps) {
  if (loading) {
    return (
      <div className="featured-channel surface" aria-busy="true">
        <div className="muted">Loading featured channel…</div>
      </div>
    );
  }

  if (!channel) {
    return (
      <div className="featured-channel surface">
        <div className="featured-channel__content">
          <span className="featured-channel__eyebrow muted">Featured</span>
          <h2>No featured broadcast yet</h2>
          <p className="muted">
            Once BitRiver highlights a standout creator, their stream will appear here so you can jump in instantly.
          </p>
        </div>
      </div>
    );
  }

  const previewImage = channel.profile.bannerUrl ?? channel.profile.avatarUrl;
  const followerLabel = `${channel.followerCount.toLocaleString()} follower${channel.followerCount === 1 ? "" : "s"}`;

  return (
    <article className="featured-channel surface">
      <div className="featured-channel__media">
        {previewImage ? (
          <img src={previewImage} alt={`${channel.owner.displayName} channel artwork`} />
        ) : (
          <div className="featured-channel__media-fallback" aria-hidden="true" />
        )}
        <div className="overlay overlay--top overlay--scrim">
          {channel.live ? <span className="badge badge--live">Live</span> : <span className="badge">Offline</span>}
          <span className="overlay__meta">{followerLabel}</span>
        </div>
      </div>
      <div className="featured-channel__content">
        <span className="featured-channel__eyebrow muted">Featured</span>
        <h2 className="featured-channel__title">{channel.channel.title}</h2>
        <p className="featured-channel__subtitle muted">{channel.owner.displayName}</p>
        <p className="muted">
          {channel.profile.bio ?? "Get to know this creator’s story and tune into their latest broadcast."}
        </p>
        <div className="tag-list">
          {channel.channel.category && <span className="tag">{channel.channel.category}</span>}
          {channel.channel.tags.slice(0, 3).map((tag) => (
            <span key={tag} className="tag">
              #{tag}
            </span>
          ))}
        </div>
        <div className="featured-channel__actions">
          <Link className="primary-button" href={`/channels/${channel.channel.id}`}>
            Watch now
          </Link>
          <Link className="secondary-button" href={`/channels/${channel.channel.id}`}>
            View channel
          </Link>
        </div>
      </div>
    </article>
  );
}
