"use client";

import Image from "next/image";
import Link from "next/link";
import { FollowingList } from "./following/FollowingList";
import {
  FOLLOWING_COPY,
  FollowingEmptyPrompt,
  FollowingErrorBlock,
  FollowingLoadingBlock,
  FollowingUnauthenticatedPrompt,
} from "./following/FollowingState";
import { useFollowingChannels } from "./following/useFollowingChannels";
import { useAuth } from "../hooks/useAuth";
import type { DirectoryChannel } from "../lib/viewer-api";

const REFRESH_INTERVAL_MS = 30_000;

export function FollowingSidebar() {
  const { user, loading: authLoading } = useAuth();
  const { channels, status, reload, error } = useFollowingChannels({
    isAuthenticated: Boolean(user),
    authLoading,
    refreshIntervalMs: REFRESH_INTERVAL_MS,
  });

  const getSummary = () => {
    if (status === "loading") {
      return FOLLOWING_COPY.loading;
    }
    if (status === "error") {
      return error ?? FOLLOWING_COPY.error;
    }
    if (status === "unauthenticated") {
      return FOLLOWING_COPY.unauthenticated;
    }
    return channels.length > 0 ? `${channels.length} creators` : "No channels yet";
  };

  const renderAvatar = (entry: DirectoryChannel) => {
    const avatar = entry.profile.avatarUrl ?? entry.profile.bannerUrl;
    const ownerInitial = entry.owner.displayName.charAt(0).toUpperCase() || "B";
    return (
      <div className="following-sidebar__avatar" aria-hidden="true">
        {avatar ? (
          <Image
            src={avatar}
            alt=""
            width={40}
            height={40}
            sizes="40px"
            className="following-sidebar__avatar-image"
          />
        ) : (
          <span>{ownerInitial}</span>
        )}
        <span
          className={`following-sidebar__status ${entry.live ? "following-sidebar__status--live" : "following-sidebar__status--offline"}`}
          aria-label={entry.live ? "Live" : "Offline"}
        />
      </div>
    );
  };

  return (
    <div className="following-sidebar">
      <header className="following-sidebar__header">
        <div>
          <p className="following-sidebar__eyebrow">Following</p>
          <h4>Creators you follow</h4>
        </div>
        <span className="following-sidebar__summary muted">{getSummary()}</span>
      </header>

      {status === "loading" ? (
        <FollowingLoadingBlock className="following-sidebar__state muted" />
      ) : status === "unauthenticated" ? (
        <FollowingUnauthenticatedPrompt className="following-sidebar__state following-sidebar__state--empty stack" />
      ) : status === "error" ? (
        <FollowingErrorBlock
          className="following-sidebar__state following-sidebar__state--error"
          onRetry={() => {
            void reload();
          }}
        />
      ) : status === "empty" ? (
        <FollowingEmptyPrompt className="following-sidebar__state following-sidebar__state--empty" />
      ) : (
        <FollowingList
          as="ul"
          channels={channels}
          className="following-sidebar__list"
          itemClassName="following-sidebar__list-item"
          renderItem={(entry) => (
            <Link href={`/channels/${entry.channel.id}`} className="following-sidebar__link">
              {renderAvatar(entry)}
              <div className="following-sidebar__meta">
                <strong>{entry.owner.displayName}</strong>
                <span className="muted">
                  {entry.channel.category ?? "Variety"}
                  {entry.channel.category && entry.channel.liveState ? " • " : ""}
                  {entry.channel.liveState}
                </span>
              </div>
            </Link>
          )}
        />
      )}

      <p className="following-sidebar__footnote muted">
        Following list updates automatically when a creator goes live.
      </p>
    </div>
  );
}
