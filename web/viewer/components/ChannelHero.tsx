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

export type ChannelHeaderProps = {
  data: ChannelPlaybackResponse;
  onFollowChange?: (state: FollowState) => void;
  onSubscriptionChange?: (state: SubscriptionState) => void;
};

export function ChannelHeader({ data, onFollowChange, onSubscriptionChange }: ChannelHeaderProps) {
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
    <section className="channel-header surface stack" aria-labelledby="channel-title">
      <header className="channel-header__meta stack" style={{ gap: "0.5rem" }}>
        <div className="channel-header__title stack" style={{ gap: "0.35rem" }}>
          <p className="muted">{data.owner.displayName}</p>
          <h1 id="channel-title">{data.channel.title}</h1>
        </div>
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
      <div className="channel-header__actions">
        <div className="channel-header__buttons">
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
      </div>
      <p className="muted" role="status">
        {status ??
          (data.live
            ? "Enjoy low-latency playback powered by the ingest pipeline."
            : "Offline for now – follow to be notified when the stream returns.")}
      </p>
    </section>
  );
}

export function ChannelAboutPanel({ data }: { data: ChannelPlaybackResponse }) {
  return (
    <section className="channel-about surface stack">
      {data.profile.bannerUrl && (
        <img
          src={data.profile.bannerUrl}
          alt={`${data.owner.displayName} channel art`}
          className="channel-about__banner"
        />
      )}
      <div className="stack" style={{ gap: "0.5rem" }}>
        <h3>About {data.owner.displayName}</h3>
        {data.profile.bio ? (
          <p className="muted">{data.profile.bio}</p>
        ) : (
          <p className="muted">The broadcaster hasn&apos;t shared a bio yet.</p>
        )}
      </div>
      <div className="channel-about__details">
        <dl>
          <dt>Channel created</dt>
          <dd>{new Date(data.channel.createdAt).toLocaleString()}</dd>
        </dl>
        <dl>
          <dt>Last updated</dt>
          <dd>{new Date(data.channel.updatedAt).toLocaleString()}</dd>
        </dl>
        {data.channel.category && (
          <dl>
            <dt>Category</dt>
            <dd>{data.channel.category}</dd>
          </dl>
        )}
      </div>
      {data.playback?.renditions && data.playback.renditions.length > 0 && (
        <div className="channel-about__renditions stack">
          <h4>Available renditions</h4>
          <ul>
            {data.playback.renditions.map((rendition) => (
              <li key={rendition.name}>
                <strong>{rendition.name}</strong> · {rendition.manifestUrl}
                {rendition.bitrate && ` · ${Math.round(rendition.bitrate / 1000)} kbps`}
              </li>
            ))}
          </ul>
        </div>
      )}
    </section>
  );
}
