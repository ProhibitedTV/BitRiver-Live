import Image from "next/image";
import Link from "next/link";
import { FollowingList } from "./following/FollowingList";
import {
  FOLLOWING_COPY,
  FollowingEmptyPrompt,
  FollowingLoadingBlock,
  FollowingStatus,
  FollowingUnauthenticatedPrompt,
} from "./following/FollowingState";
import type { DirectoryChannel } from "../lib/viewer-api";

interface FollowingRailProps {
  channels: DirectoryChannel[];
  loading?: boolean;
  isAuthenticated?: boolean;
}

export function FollowingRail({ channels, loading = false, isAuthenticated = false }: FollowingRailProps) {
  const status: FollowingStatus = loading ? "loading" : !isAuthenticated ? "unauthenticated" : channels.length === 0 ? "empty" : "ready";

  return (
    <section className="following-rail surface">
      <header className="following-rail__header">
        <div className="stack">
          <span className="following-rail__eyebrow muted">Following</span>
          <h3>Catch up with your creators</h3>
        </div>
        {status === "ready" && <span className="muted">{FOLLOWING_COPY.summaryLiveNow(channels.length)}</span>}
      </header>
      {status === "loading" ? (
        <FollowingLoadingBlock className="muted" />
      ) : status === "unauthenticated" ? (
        <FollowingUnauthenticatedPrompt className="stack" />
      ) : status === "empty" ? (
        <FollowingEmptyPrompt className="muted" />
      ) : (
        <FollowingList
          channels={channels}
          className="following-rail__scroller"
          role="list"
          itemRole="listitem"
          renderItem={(entry) => {
            const avatar = entry.profile.avatarUrl ?? entry.profile.bannerUrl;
            const ownerInitial = entry.owner.displayName.charAt(0).toUpperCase() || "B";
            return (
              <Link href={`/channels/${entry.channel.id}`} className="following-card">
                <div className="following-card__avatar">
                  {avatar ? (
                    <Image
                      src={avatar}
                      alt={entry.owner.displayName}
                      width={56}
                      height={56}
                      sizes="56px"
                      className="following-card__avatar-image"
                      priority
                    />
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
          }}
        />
      )}
    </section>
  );
}
