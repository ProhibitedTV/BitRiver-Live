"use client";

import Image from "next/image";
import { useEffect, useMemo, useRef, useState } from "react";
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
import { TipDrawer } from "./TipDrawer";
import { DonationQRCode } from "./DonationQRCode";

export type ChannelHeaderProps = {
  data: ChannelPlaybackResponse;
  onFollowChange?: (state: FollowState) => void;
  onSubscriptionChange?: (state: SubscriptionState) => void;
};

export function ChannelHeader({ data, onFollowChange, onSubscriptionChange }: ChannelHeaderProps) {
  const { user, signIn } = useAuth();
  const [follow, setFollow] = useState<FollowState>(data.follow);
  const initialSubscription: SubscriptionState = data.subscription ?? {
    subscribed: false,
    subscribers: 0
  };
  const [subscription, setSubscription] = useState<SubscriptionState>(initialSubscription);
  const [status, setStatus] = useState<string | undefined>();
  const [loading, setLoading] = useState(false);
  const [subscriptionLoading, setSubscriptionLoading] = useState(false);
  const [tipOpen, setTipOpen] = useState(false);
  const tipTriggerRef = useRef<HTMLButtonElement | null>(null);
  const [copiedLink, setCopiedLink] = useState(false);
  const donationAddresses = data.donationAddresses ?? [];
  const isOwner = user?.id === data.owner.id;

  useEffect(() => {
    setFollow(data.follow);
  }, [data.follow]);

  useEffect(() => {
    const nextSubscription: SubscriptionState = data.subscription ?? {
      subscribed: false,
      subscribers: 0
    };
    setSubscription(nextSubscription);
  }, [data.subscription]);

  const handleToggleFollow = async () => {
    if (!user) {
      setStatus("Redirecting to sign in...");
      void signIn();
      return;
    }
    if (isOwner) {
      setStatus("You manage this channel, so following is disabled for owners.");
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
      setStatus("Redirecting to sign in...");
      void signIn();
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

  const handleOpenTip = () => {
    if (!user) {
      setStatus("Sign in from the header to send a tip to this channel.");
      return;
    }
    setStatus(undefined);
    setTipOpen(true);
  };

  const handleTipSuccess = (message: string) => {
    setStatus(message);
    setTipOpen(false);
  };

  const handleShare = async () => {
    const shareUrl = typeof window !== "undefined" ? window.location.href : "";
    if (!shareUrl) return;
    try {
      await navigator.clipboard.writeText(shareUrl);
      setCopiedLink(true);
      setStatus("Channel link copied.");
      setTimeout(() => setCopiedLink(false), 2500);
    } catch {
      setStatus("Copy isn't supported in this browser.");
    }
  };

  const viewerCount = useMemo(() => {
    if (data.viewerCount !== undefined) {
      return data.viewerCount;
    }
    return undefined;
  }, [data.viewerCount]);

  return (
    <section className="channel-hero surface" aria-labelledby="channel-title">
      <header className="channel-hero__top">
        <div className="channel-hero__identity">
          <div className="channel-hero__eyebrow">
            {data.live ? <span className="pill pill--live">Live</span> : <span className="pill">Offline</span>}
            {viewerCount !== undefined && (
              <span className="pill pill--ghost" aria-label="Current viewers">
                <svg viewBox="0 0 20 20" aria-hidden="true" focusable="false">
                  <path
                    fill="currentColor"
                    d="M10 4.5c3.94 0 7.23 2.35 8.86 5.59C17.23 13.33 13.94 15.67 10 15.67S2.77 13.33 1.14 10.09C2.77 6.85 6.06 4.5 10 4.5Zm0 1.8c-2.8 0-5.22 1.63-6.46 3.79 1.24 2.15 3.66 3.78 6.46 3.78s5.22-1.63 6.46-3.78C15.22 7.93 12.8 6.3 10 6.3Zm0 1.45a2.25 2.25 0 1 1 0 4.5 2.25 2.25 0 0 1 0-4.5Z"
                  />
                </svg>
                {viewerCount.toLocaleString()} watching
              </span>
            )}
          </div>
          <div className="channel-hero__title">
            <p className="muted">{data.owner.displayName}</p>
            <h1 id="channel-title">{data.channel.title}</h1>
          </div>
        </div>
        <div className="channel-hero__actions" aria-label="Channel actions">
          <button
            className="pill-action"
            onClick={handleToggleFollow}
            disabled={loading || isOwner}
            aria-pressed={follow.following}
            aria-label={`${follow.following ? "Following" : "Follow"} - ${follow.followers.toLocaleString()} followers`}
            type="button"
          >
            <svg viewBox="0 0 20 20" aria-hidden="true" focusable="false">
              <path
                fill="currentColor"
                d="M10 17.5S3 12.45 3 7.7A4.7 4.7 0 0 1 10 4a4.7 4.7 0 0 1 7 3.7c0 4.75-7 9.8-7 9.8Z"
              />
            </svg>
            {follow.following ? "Following" : "Follow"}
            <span className="pill-action__meta">{follow.followers.toLocaleString()}</span>
          </button>
          <button
            className="pill-action"
            onClick={handleToggleSubscription}
            disabled={subscriptionLoading}
            aria-pressed={subscription.subscribed}
            aria-label={`${subscription.subscribed ? `Subscribed${subscription.tier ? ` - ${subscription.tier}` : ""}` : "Subscribe"} - ${subscription.subscribers.toLocaleString()} subscribers`}
            type="button"
          >
            {subscription.subscribed ? `Subscribed${subscription.tier ? ` - ${subscription.tier}` : ""}` : "Subscribe"}
          </button>
          <button
            className="pill-action pill-action--accent"
            onClick={handleOpenTip}
            aria-label="Send a tip"
            ref={tipTriggerRef}
            type="button"
          >
            <svg viewBox="0 0 20 20" aria-hidden="true" focusable="false">
              <path
                fill="currentColor"
                d="M10 2.75a2 2 0 0 1 2 2V6h1.25a3 3 0 0 1 3 3v.92a3.5 3.5 0 0 1-.58 1.93l-1.85 2.78a3.5 3.5 0 0 1-2.91 1.57H8a2 2 0 0 1-2-2v-4.07c0-.58.17-1.15.5-1.63L8.8 4.6A2 2 0 0 1 10 2.75ZM4 8.5h1v7H4a1 1 0 0 1-1-1v-5a1 1 0 0 1 1-1Z"
              />
            </svg>
            Send a tip
          </button>
          <button className="pill-action pill-action--ghost" type="button" onClick={handleShare}>
            <svg viewBox="0 0 20 20" aria-hidden="true" focusable="false">
              <path
                fill="currentColor"
                d="M15 13.5a2.5 2.5 0 1 1-2.13 3.75l-4.6-2.53a2.5 2.5 0 0 1 0-2.44l4.6-2.53a2.5 2.5 0 1 1 .54 1.3l-4.6 2.53c-.1.06-.16.16-.16.27 0 .11.06.21.16.27l4.6 2.53c.35-.52.94-.85 1.59-.85ZM6 4a2 2 0 1 1-1.95 2.46l4.77 2.63a2.5 2.5 0 0 1 0 2.44l-4.77 2.63a2 2 0 1 1-.54-1.1l4.76-2.63c.1-.06.17-.16.17-.27 0-.11-.06-.21-.16-.27L3.5 7.17A2 2 0 0 1 6 4Z"
              />
            </svg>
            {copiedLink ? "Copied" : "Share"}
          </button>
        </div>
      </header>
      <div className="channel-hero__meta-row">
        <div className="tag-list">
          {data.channel.category && <span className="tag">{data.channel.category}</span>}
          {data.channel.tags.map((tag) => (
            <span key={tag} className="tag">
              #{tag}
            </span>
          ))}
        </div>
        <div className="channel-hero__status">
          <p className="muted" role="status">
            {status ??
              (isOwner
                ? "Owner view. Followers will see public updates here."
                : data.live
                  ? "Live now."
                  : "Offline. Follow for updates.")}
          </p>
        </div>
      </div>
      <div className="channel-hero__stats" aria-label="Channel stats">
        <dl>
          <dt>Followers</dt>
          <dd>{follow.followers.toLocaleString()}</dd>
        </dl>
        <dl>
          <dt>Subscribers</dt>
          <dd>{subscription.subscribers.toLocaleString()}</dd>
        </dl>
        <dl>
          <dt>Status</dt>
          <dd>{data.live ? "Live" : "Offline"}</dd>
        </dl>
      </div>
      <TipDrawer
        open={tipOpen}
        channelId={data.channel.id}
        channelTitle={data.channel.title}
        donationAddresses={donationAddresses}
        onClose={() => setTipOpen(false)}
        onSuccess={handleTipSuccess}
        returnFocusRef={tipTriggerRef}
      />
    </section>
  );
}

export function ChannelAboutPanel({ data }: { data: ChannelPlaybackResponse }) {
  const [copiedAddress, setCopiedAddress] = useState<string | null>(null);
  const [copyStatus, setCopyStatus] = useState<
    { type: "success" | "error"; message: string } | undefined
  >();
  const donations = data.donationAddresses ?? [];
  const socialLinks = data.profile.socialLinks ?? [];

  const handleCopy = async (address: string, currency: string) => {
    const currencyLabel = currency.toUpperCase();
    if (
      typeof navigator === "undefined" ||
      !navigator.clipboard ||
      typeof navigator.clipboard.writeText !== "function"
    ) {
      setCopiedAddress(null);
      setCopyStatus({
        type: "error",
        message: "Copy isn't supported in this browser."
      });
      return;
    }
    try {
      await navigator.clipboard.writeText(address);
      setCopiedAddress(address);
      setCopyStatus({ type: "success", message: `${currencyLabel} address copied to clipboard.` });
    } catch {
      setCopiedAddress(null);
      setCopyStatus({
        type: "error",
        message: "Unable to copy the address. Try again."
      });
    }
  };

  return (
    <section className="channel-about surface stack">
      {data.profile.bannerUrl && (
        <Image
          src={data.profile.bannerUrl}
          alt={`${data.owner.displayName} channel art`}
          className="channel-about__banner"
          width={1200}
          height={200}
          priority
        />
      )}
      <div className="stack" style={{ gap: "0.5rem" }}>
        <h3>About</h3>
        {data.profile.bio ? (
          <p className="muted">{data.profile.bio}</p>
        ) : (
          <p className="muted">No bio yet.</p>
        )}
      </div>
      <div className="channel-about__social stack">
        <h4>Links</h4>
        {socialLinks.length > 0 ? (
          <ul className="social-links" role="list">
            {socialLinks.map((link, index) => {
              const label = link.platform?.trim() || link.url;
              const key = `${link.platform}-${link.url}-${index}`;
              return (
                <li key={key} className="social-link">
                  <a href={link.url} target="_blank" rel="noreferrer">
                    <span className="social-link__platform">{label}</span>
                    <span className="social-link__url muted">{link.url}</span>
                    <svg viewBox="0 0 20 20" aria-hidden="true" focusable="false">
                      <path
                        fill="currentColor"
                        d="M11.5 4a.5.5 0 0 0 0 1h2.793l-7.146 7.146a.5.5 0 1 0 .707.708L15 5.707V8.5a.5.5 0 0 0 1 0v-4A.5.5 0 0 0 15.5 4h-4Z"
                      />
                      <path
                        fill="currentColor"
                        d="M5 6a1 1 0 0 0-1 1v8a1 1 0 0 0 1 1h8a1 1 0 0 0 1-1v-3a.5.5 0 0 0-1 0v3H5V7h3a.5.5 0 0 0 0-1H5Z"
                      />
                    </svg>
                  </a>
                </li>
              );
            })}
          </ul>
        ) : (
          <p className="muted">No links yet.</p>
        )}
      </div>
      {data.playback?.renditions && data.playback.renditions.length > 0 && (
        <div className="channel-about__renditions stack">
          <h4>Renditions</h4>
          <ul>
            {data.playback.renditions.map((rendition) => (
              <li key={rendition.name}>
                <strong>{rendition.name}</strong> - {rendition.manifestUrl}
                {rendition.bitrate && ` - ${Math.round(rendition.bitrate / 1000)} kbps`}
              </li>
            ))}
          </ul>
        </div>
      )}
      <div className="channel-about__donations stack">
        <h4>Support</h4>
        {donations.length > 0 ? (
          <>
            <p className="muted">Donation addresses.</p>
            <ul className="donation-list" role="list">
              {donations.map((donation, index) => {
                const currencyLabel = donation.currency
                  ? donation.currency.toUpperCase()
                  : "DONATION";
                const key = `${donation.currency}-${donation.address}-${index}`;
                const note = donation.note?.trim();
                const isCopied = copiedAddress === donation.address;
                return (
                  <li
                    key={key}
                    className={`donation-item${isCopied ? " donation-item--copied" : ""}`}
                  >
                    <div className="donation-item__icon" aria-hidden="true">
                      {currencyLabel.slice(0, 4)}
                    </div>
                    <div className="donation-item__meta">
                      <div className="donation-item__heading">
                        <strong>{currencyLabel}</strong>
                        {note && <span className="donation-item__note muted">{note}</span>}
                      </div>
                      <code>{donation.address}</code>
                    </div>
                    <div className="donation-item__qr">
                      <DonationQRCode
                        value={donation.address}
                        label={`${currencyLabel} address QR code`}
                      />
                    </div>
                    <button
                      type="button"
                      className="donation-item__copy-button"
                      onClick={() => handleCopy(donation.address, currencyLabel)}
                      aria-label={`Copy ${currencyLabel} address`}
                    >
                      <svg viewBox="0 0 20 20" aria-hidden="true" focusable="false">
                        <path
                          fill="currentColor"
                          d="M6 3h8a2 2 0 0 1 2 2v11H6a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2Zm0 2v9h8V5H6Zm4-4h6v2h-6V1Z"
                        />
                      </svg>
                      <span className="sr-only">Copy {currencyLabel} address</span>
                    </button>
                  </li>
                );
              })}
            </ul>
          </>
        ) : (
          <p className="muted">No donation addresses yet.</p>
        )}
        {copyStatus && (
          <p
            className={`donation-copy-status${copyStatus.type === "error" ? " donation-copy-status--error" : ""}`}
            role="status"
            aria-live="polite"
          >
            {copyStatus.message}
          </p>
        )}
      </div>
    </section>
  );
}
