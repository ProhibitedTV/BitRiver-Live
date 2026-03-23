import Image from "next/image";
import Link from "next/link";
import { ChannelAvatar } from "./channel/ChannelAvatar";
import { ChannelCardShell } from "./channel/ChannelCardShell";
import { ChannelMetaChips } from "./channel/ChannelMetaChips";
import { ChannelStatusBadge } from "./channel/ChannelStatusBadge";
import { formatFollowerLabel, formatViewerLabel, getChannelAvatarImage, getChannelPreviewImage } from "../lib/channel-presenters";
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
      {channels.map((entry, index) => {
        const createdAt = new Date(entry.channel.createdAt).toLocaleDateString(undefined, { timeZone: "UTC" });
        const previewImage = getChannelPreviewImage(entry);
        const followerLabel = formatFollowerLabel(entry.followerCount);
        const viewerOverlayLabel = formatViewerLabel(entry.viewerCount ?? 0);
        const isLive = entry.live;

        return (
          <ChannelCardShell key={entry.channel.id} className="directory-card">
            <Link href={`/channels/${entry.channel.id}`} className="directory-card__link">
              <div className="directory-card__preview">
                {previewImage ? (
                  <Image
                    src={previewImage}
                    alt={`${entry.owner.displayName} channel artwork`}
                    className="directory-card__media"
                    fill
                    sizes="(min-width: 1280px) 25vw, (min-width: 768px) 33vw, 100vw"
                    priority={index < 1}
                    loading={index < 1 ? undefined : "lazy"}
                  />
                ) : (
                  <div className="directory-card__preview-fallback" aria-hidden="true" />
                )}
                <div className="overlay overlay--top overlay--scrim overlay--glow">
                  <div className="overlay__status">
                    <ChannelStatusBadge live={isLive} />
                    <span className="overlay__meta">{isLive ? viewerOverlayLabel : followerLabel}</span>
                  </div>
                  {entry.channel.category && <span className="pill pill--frost">{entry.channel.category}</span>}
                </div>
                <div className="overlay overlay--bottom overlay--scrim overlay--frost">
                  <div className="overlay__identity">
                    <ChannelAvatar displayName={entry.owner.displayName} avatarUrl={getChannelAvatarImage(entry)} />
                  </div>
                  <div className="overlay__tags">
                    {entry.channel.tags.slice(0, 2).map((tag) => (
                      <span key={tag} className="pill pill--tag">
                        #{tag}
                      </span>
                    ))}
                  </div>
                </div>
              </div>
              <div className="directory-card__content">
                <div className="directory-card__meta-row">
                  <ChannelStatusBadge live={isLive} />
                  <ChannelMetaChips
                    live={isLive}
                    viewerCount={entry.viewerCount}
                    followerCount={entry.followerCount}
                    showFollowerSummary
                    followerSummaryPrefix="Followers: "
                    category={entry.channel.category ?? "Streaming"}
                  />
                </div>

                <div className="directory-card__header">
                  <h3 className="directory-card__title">{entry.channel.title}</h3>
                  <span className="directory-card__subtitle muted">{entry.owner.displayName}</span>
                </div>

                {entry.profile.bio && <p className="directory-card__description muted">{entry.profile.bio}</p>}

                <div className="directory-card__tags-row">
                  {entry.channel.tags.slice(0, 3).map((tag) => (
                    <span key={tag} className="pill pill--tag">
                      #{tag}
                    </span>
                  ))}
                  {entry.channel.tags.length === 0 && <span className="muted">No tags yet</span>}
                </div>
              </div>
            </Link>
            <footer className="directory-card__footer">
              <span className="muted">Created {createdAt}</span>
              <Link className="secondary-button" href={`/channels/${entry.channel.id}`}>
                View channel
              </Link>
            </footer>
          </ChannelCardShell>
        );
      })}
    </section>
  );
}
