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
import type { FollowingStatus } from "./following/FollowingState";

const REFRESH_INTERVAL_MS = 30_000;

interface FollowingSidebarContentProps {
  channels: DirectoryChannel[];
  status: FollowingStatus;
  onRetry?: () => void;
}

export function FollowingSidebarContent({ channels, status, onRetry }: FollowingSidebarContentProps) {
  const summary = (() => {
    if (status === "ready") {
      return channels.length > 0 ? FOLLOWING_COPY.summaryFollowed(channels.length) : FOLLOWING_COPY.empty;
    }
    return null;
  })();

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
        <div className="stack stack--2xs">
          <div className="following-sidebar__title-row">
            <p className="following-sidebar__eyebrow">Live network</p>
            {status === "ready" && <span className="following-sidebar__count">{channels.length}</span>}
          </div>
          <h3>Following</h3>
        </div>
        {summary ? <p className="following-sidebar__summary muted">{summary}</p> : null}
      </header>

      {status === "loading" ? (
        <FollowingLoadingBlock className="following-sidebar__state muted" />
      ) : status === "unauthenticated" ? (
        <FollowingUnauthenticatedPrompt className="following-sidebar__state following-sidebar__state--prompt stack" />
      ) : status === "error" ? (
        <FollowingErrorBlock
          className="following-sidebar__state following-sidebar__state--error"
          onRetry={onRetry}
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
                  {entry.channel.category && entry.channel.liveState ? " - " : ""}
                  {entry.channel.liveState}
                </span>
              </div>
            </Link>
          )}
        />
      )}
    </div>
  );
}

export function FollowingSidebar() {
  const { user, loading: authLoading } = useAuth();
  const { channels, status, reload } = useFollowingChannels({
    isAuthenticated: Boolean(user),
    authLoading,
    refreshIntervalMs: REFRESH_INTERVAL_MS,
  });

  return (
    <FollowingSidebarContent
      channels={channels}
      status={status}
      onRetry={() => {
        void reload();
      }}
    />
  );
}
