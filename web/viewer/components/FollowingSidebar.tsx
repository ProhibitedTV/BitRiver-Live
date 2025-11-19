"use client";

import Image from "next/image";
import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
import type { DirectoryChannel } from "../lib/viewer-api";
import { fetchFollowingChannels } from "../lib/viewer-api";

interface FetchState {
  status: "idle" | "loading" | "loaded" | "error";
  error?: string;
}

export function FollowingSidebar() {
  const [channels, setChannels] = useState<DirectoryChannel[]>([]);
  const [fetchState, setFetchState] = useState<FetchState>({ status: "idle" });
  const mountedRef = useRef(true);

  const loadFollowing = useCallback(async () => {
    setFetchState({ status: "loading" });
    try {
      const response = await fetchFollowingChannels();
      if (!mountedRef.current) {
        return;
      }
      setChannels(response.channels);
      setFetchState({ status: "loaded" });
    } catch (error) {
      if (!mountedRef.current) {
        return;
      }
      const message = error instanceof Error ? error.message : "Unable to load following";
      setFetchState({ status: "error", error: message });
    }
  }, []);

  useEffect(() => {
    mountedRef.current = true;
    loadFollowing();
    return () => {
      mountedRef.current = false;
    };
  }, [loadFollowing]);

  const getSummary = () => {
    if (fetchState.status === "loading") {
      return "Checking who is live…";
    }
    if (fetchState.status === "error") {
      return fetchState.error ?? "Unable to load following";
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

      {fetchState.status === "loading" ? (
        <p className="following-sidebar__state muted">Checking which creators are live…</p>
      ) : fetchState.status === "error" ? (
        <div className="following-sidebar__state following-sidebar__state--error" role="status">
          <p>We couldn&rsquo;t load your followed channels.</p>
          <button type="button" onClick={loadFollowing} className="following-sidebar__retry">
            Try again
          </button>
        </div>
      ) : channels.length === 0 ? (
        <p className="following-sidebar__state following-sidebar__state--empty">
          You&rsquo;re not following any channels yet. Follow a creator to see their live status at a glance.
        </p>
      ) : (
        <ul className="following-sidebar__list">
          {channels.map((entry) => (
            <li key={entry.channel.id} className="following-sidebar__list-item">
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
            </li>
          ))}
        </ul>
      )}

      <p className="following-sidebar__footnote muted">
        Following list updates automatically when a creator goes live.
      </p>
    </div>
  );
}
