import Image from "next/image";
import Link from "next/link";
import { ChannelAvatar } from "./channel/ChannelAvatar";
import { ChannelStatusBadge } from "./channel/ChannelStatusBadge";
import { formatViewerLabel, getChannelAvatarImage, getChannelPreviewImage } from "../lib/channel-presenters";
import type { DirectoryChannel } from "../lib/viewer-api";

interface LiveNowGridProps {
  channels: DirectoryChannel[];
  loading?: boolean;
}

export function LiveNowGrid({ channels, loading = false }: LiveNowGridProps) {
  if (loading) {
    return (
      <div className="state-panel state-panel--loading" aria-busy="true">
        <strong>Loading live channels</strong>
        <p className="muted">Checking which creators are currently on air.</p>
      </div>
    );
  }

  if (channels.length === 0) {
    return (
      <div className="state-panel">
        <strong>Nobody is live right now</strong>
        <p className="muted">As soon as creators go live, their broadcasts will show up here.</p>
        <div className="browse-actions">
          <Link href="/browse" className="secondary-button">
            Browse full directory
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div className="grid live-now-grid">
      {channels.map((entry, index) => {
        const previewImage = getChannelPreviewImage(entry);
        return (
          <Link key={entry.channel.id} className="live-card" href={`/channels/${entry.channel.id}`}>
            <div className="live-card__media">
              {previewImage ? (
                <Image
                  src={previewImage}
                  alt={`${entry.owner.displayName} channel artwork`}
                  fill
                  sizes="(min-width: 1280px) 25vw, (min-width: 768px) 33vw, 100vw"
                  className="live-card__media-image"
                  priority={index < 1}
                  loading={index < 1 ? undefined : "lazy"}
                />
              ) : (
                <div className="live-card__media-fallback" aria-hidden="true" />
              )}
              <div className="overlay overlay--top overlay--scrim overlay--glow">
                <div className="overlay__status">
                  <ChannelStatusBadge live />
                  <span className="overlay__meta">{formatViewerLabel(entry.viewerCount ?? 0)}</span>
                </div>
                {entry.channel.category && <span className="pill pill--frost">{entry.channel.category}</span>}
              </div>
              <div className="overlay overlay--bottom overlay--scrim overlay--frost">
                <div className="overlay__identity">
                  <ChannelAvatar displayName={entry.owner.displayName} avatarUrl={getChannelAvatarImage(entry)} />
                  <div className="overlay__byline">
                    <span className="overlay__name">{entry.owner.displayName}</span>
                    <span className="overlay__meta overlay__meta--muted">
                      {entry.channel.tags[0] ? `#${entry.channel.tags[0]}` : "Live"}
                    </span>
                  </div>
                </div>
                {entry.channel.tags[1] && <span className="pill pill--tag">#{entry.channel.tags[1]}</span>}
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
