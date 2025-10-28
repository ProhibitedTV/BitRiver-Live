"use client";

import { useState } from "react";
import { useAuth } from "../hooks/useAuth";
import type {
  ChannelPlaybackResponse,
  FollowState,
  SubscriptionState
} from "../lib/viewer-api";
import {
  followChannel,
  subscribeChannel,
  unfollowChannel,
  unsubscribeChannel
} from "../lib/viewer-api";

export function ChannelHero({
  data,
  onFollowChange,
  onSubscriptionChange
}: {
  data: ChannelPlaybackResponse;
  onFollowChange?: (state: FollowState) => void;
  onSubscriptionChange?: (state: SubscriptionState) => void;
}) {
  const { user } = useAuth();
  const [follow, setFollow] = useState<FollowState>(data.follow);
  const initialSubscription: SubscriptionState = data.subscription ?? {
    subscribed: false,
    subscribers: 0
  };
  const [subscription, setSubscription] = useState<SubscriptionState>(initialSubscription);
  const [status, setStatus] = useState<string | undefined>();
  const [loading, setLoading] = useState(false);
  const [subscriptionLoading, setSubscriptionLoading] = useState(false);

  const handleToggleFollow = async () => {
    if (!user) {
      setStatus("Sign in from the header to follow this channel.");
      return;
    }
    try {
      setLoading(true);
      setStatus(undefined);
      const next = follow.following
        ? await unfollowChannel(data.channel.id)
        : await followChannel(data.channel.id);
      setFollow(next);
      onFollowChange?.(next);
    } catch (err) {
      setStatus(err instanceof Error ? err.message : "Unable to update follow state");
    } finally {
      setLoading(false);
    }
  };

  const handleToggleSubscription = async () => {
    if (!user) {
      setStatus("Sign in from the header to subscribe to this channel.");
      return;
    }
    try {
      setSubscriptionLoading(true);
      setStatus(undefined);
      const next = subscription.subscribed
        ? await unsubscribeChannel(data.channel.id)
        : await subscribeChannel(data.channel.id);
      setSubscription(next);
      onSubscriptionChange?.(next);
    } catch (err) {
      setStatus(err instanceof Error ? err.message : "Unable to update subscription");
    } finally {
      setSubscriptionLoading(false);
    }
  };

  return (
    <section className="hero">
      <div className="stack">
        <header className="stack">
          <h1>{data.channel.title}</h1>
          <div className="tag-list">
            {data.live && <span className="badge">Live now</span>}
            {data.channel.category && <span className="tag">{data.channel.category}</span>}
            {data.channel.tags.map((tag) => (
              <span key={tag} className="tag">
                #{tag}
              </span>
            ))}
          </div>
        </header>
          <div className="surface stack">
            <div className="stack" style={{ gap: "0.35rem" }}>
              <strong>{data.owner.displayName}</strong>
              {data.profile.bio && <p className="muted">{data.profile.bio}</p>}
            </div>
            <div className="nav-links" style={{ justifyContent: "flex-start" }}>
              <button className="primary-button" onClick={handleToggleFollow} disabled={loading}>
                {follow.following ? "Following" : "Follow"} · {follow.followers} supporter
                {follow.followers === 1 ? "" : "s"}
              </button>
              <button
                className="secondary-button"
                onClick={handleToggleSubscription}
                disabled={subscriptionLoading}
                aria-pressed={subscription.subscribed}
              >
                {subscription.subscribed ? "Subscribed" : "Subscribe"}
                {subscription.tier ? ` · ${subscription.tier}` : ""}
              </button>
              <span className="muted">
                {data.live
                  ? "Enjoy low-latency playback powered by the ingest pipeline."
                  : "Offline for now – follow to be notified when the stream returns."}
              </span>
            </div>
            <dl className="channel-stats" aria-label="Channel community stats">
              <div>
                <dt>Followers</dt>
                <dd>{follow.followers.toLocaleString()}</dd>
              </div>
              <div>
                <dt>Subscribers</dt>
                <dd>{subscription.subscribers.toLocaleString()}</dd>
              </div>
            </dl>
            {status && <span className="muted">{status}</span>}
          </div>
      </div>
      <aside className="stack">
        {data.profile.bannerUrl && (
          <img src={data.profile.bannerUrl} alt={`${data.owner.displayName} channel art`} />
        )}
        <div className="surface stack">
          <h3>Channel details</h3>
          <p className="muted">
            Created {new Date(data.channel.createdAt).toLocaleString()} · Updated {new Date(data.channel.updatedAt).toLocaleString()}
          </p>
        </div>
      </aside>
    </section>
  );
}
