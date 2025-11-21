"use client";

import Image from "next/image";
import { FormEvent, useEffect, useMemo, useState } from "react";
import { useAuth } from "../hooks/useAuth";
import type { ChatMessage } from "../lib/viewer-api";
import { fetchChannelChat, sendChatMessage } from "../lib/viewer-api";

const POLL_INTERVAL_MS = 10_000;

export function ChatPanel({
  channelId,
  roomId,
  viewerCount
}: {
  channelId: string;
  roomId?: string;
  viewerCount?: number;
}) {
  const { user } = useAuth();
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | undefined>();
  const [content, setContent] = useState("");
  const [sending, setSending] = useState(false);
  const [pausedForAuth, setPausedForAuth] = useState(false);
  const [authRequired, setAuthRequired] = useState(false);
  const [showPopoutDialog, setShowPopoutDialog] = useState(false);
  const [showSettings, setShowSettings] = useState(false);
  const [showAvatars, setShowAvatars] = useState(true);
  const [showTimestamps, setShowTimestamps] = useState(true);

  const isUnauthorizedError = (err: unknown) => {
    if (!(err instanceof Error)) {
      return false;
    }
    const rawMessage = err.message.trim();
    if (rawMessage === "401") {
      return true;
    }
    try {
      const parsed = JSON.parse(rawMessage);
      if (
        parsed &&
        typeof parsed === "object" &&
        "error" in parsed &&
        typeof parsed.error === "string" &&
        parsed.error.toLowerCase().includes("authentication required")
      ) {
        return true;
      }
    } catch {
      // fall through to string checks
    }
    return rawMessage.toLowerCase().includes("authentication required");
  };

  const sortedMessages = useMemo(() => {
    return [...messages].sort(
      (a, b) => new Date(a.sentAt).getTime() - new Date(b.sentAt).getTime()
    );
  }, [messages]);

  const groupedMessages = useMemo(() => {
    const groups: {
      id: string;
      userLabel: string;
      avatar?: string;
      role?: string;
      messages: ChatMessage[];
    }[] = [];
    const TIME_DELTA_MS = 2 * 60 * 1000;
    sortedMessages.forEach((message) => {
      const displayName = message.user?.displayName ?? message.user?.id ?? "Anonymous";
      const previous = groups[groups.length - 1];
      const messageDate = new Date(message.sentAt).getTime();
      const previousDate = previous?.messages.length
        ? new Date(previous.messages[previous.messages.length - 1].sentAt).getTime()
        : undefined;
      const sameUser = previous?.userLabel === displayName;
      const withinWindow = previousDate
        ? Math.abs(messageDate - previousDate) <= TIME_DELTA_MS
        : false;

      if (previous && sameUser && withinWindow) {
        previous.messages.push(message);
      } else {
        groups.push({
          id: message.id,
          userLabel: displayName,
          avatar: message.user?.avatarUrl,
          role: message.user?.role,
          messages: [message]
        });
      }
    });
    return groups;
  }, [sortedMessages]);

  useEffect(() => {
    if (pausedForAuth) {
      if (user) {
        setPausedForAuth(false);
        setAuthRequired(false);
      } else {
        setLoading(false);
        setAuthRequired(true);
      }
      return;
    }

    let cancelled = false;
    let shouldPoll = true;
    let interval: ReturnType<typeof setInterval> | undefined;

    const load = async (showSpinner: boolean) => {
      if (cancelled || !shouldPoll || pausedForAuth) {
        return;
      }
      try {
        if (showSpinner) {
          setLoading(true);
        }
        setError(undefined);
        const chatMessages = await fetchChannelChat(channelId);
        if (!cancelled) {
          setMessages(chatMessages);
          setAuthRequired(false);
        }
      } catch (err) {
        if (!cancelled) {
          if (isUnauthorizedError(err) && !user) {
            shouldPoll = false;
            setPausedForAuth(true);
            setAuthRequired(true);
            setMessages([]);
            setError(undefined);
          } else {
            setError(err instanceof Error ? err.message : "Unable to load chat");
          }
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    };

    void load(true);
    interval = setInterval(() => {
      void load(false);
    }, POLL_INTERVAL_MS);

    return () => {
      cancelled = true;
      if (interval) {
        clearInterval(interval);
      }
    };
  }, [channelId, roomId, user, pausedForAuth]);

  const handleSend = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!content.trim()) {
      return;
    }
    if (!user) {
      return;
    }

    try {
      setSending(true);
      const message = await sendChatMessage(channelId, user.id, content.trim());
      setMessages((prev) => [...prev, message]);
      setContent("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to send message");
    } finally {
      setSending(false);
    }
  };

  const isComposerDisabled = !user || sending;

  const shouldShowSignInPrompt = authRequired && !user;

  const handlePopout = () => {
    setShowPopoutDialog(true);
  };

  const openPopoutWindow = () => {
    if (typeof window === "undefined") {
      return;
    }
    window.open(window.location.href, "bitriver-chat-popout", "width=420,height=720,noopener,noreferrer");
    setShowPopoutDialog(false);
  };

  const renderSkeletons = () => (
    <ul className="chat-skeletons">
      {Array.from({ length: 4 }).map((_, index) => (
        <li key={index} className="chat-skeleton">
          <span className="chat-skeleton__avatar" aria-hidden />
          <div className="chat-skeleton__lines" aria-hidden>
            <span />
            <span />
          </div>
        </li>
      ))}
    </ul>
  );

  return (
    <section className="chat-panel" aria-live="polite">
      <header className="chat-panel__header">
        <div className="chat-panel__title">
          <h3>Live chat</h3>
          <div className="chat-panel__counts">
            {viewerCount !== undefined && (
              <span className="pill pill--ghost">{viewerCount.toLocaleString()} viewers</span>
            )}
            <span className="pill pill--ghost">{messages.length} messages</span>
          </div>
        </div>
        <div className="chat-panel__actions" aria-label="Chat actions">
          <button
            type="button"
            className="icon-button"
            aria-label="Open pop-out chat window"
            onClick={handlePopout}
          >
            <svg viewBox="0 0 20 20" aria-hidden="true" focusable="false">
              <path
                fill="currentColor"
                d="M4 4h6v2H6v8h8v-4h2v6H4V4Zm6-1.5H18v8h-2V5.91l-4.4 4.4-1.42-1.42L14.59 4H10V2.5Z"
              />
            </svg>
          </button>
          <button
            type="button"
            className="icon-button"
            aria-label={showSettings ? "Close chat settings" : "Open chat settings"}
            aria-pressed={showSettings}
            onClick={() => setShowSettings((open) => !open)}
          >
            <svg viewBox="0 0 20 20" aria-hidden="true" focusable="false">
              <path
                fill="currentColor"
                d="M9.6 2h.8l.6 1.7c.3.1.6.3.9.4l1.6-.9.6.6-.9 1.6c.1.3.3.6.4.9L16.4 8v.8l-1.7.6c-.1.3-.3.6-.4.9l.9 1.6-.6.6-1.6-.9c-.3.1-.6.3-.9.4L10.4 18h-.8l-.6-1.7c-.3-.1-.6-.3-.9-.4l-1.6.9-.6-.6.9-1.6c-.1-.3-.3-.6-.4-.9L2 10.8V10l1.7-.6c.1-.3.3-.6.4-.9L3.2 6.9l.6-.6 1.6.9c.3-.1.6-.3.9-.4L9.6 2Zm.4 5a3 3 0 1 0 0 6 3 3 0 0 0 0-6Z"
              />
            </svg>
          </button>
        </div>
      </header>
      {loading && renderSkeletons()}
      {error && <div className="surface" role="alert">{error}</div>}
      {!loading && !error && (
        <div className="chat-panel__body" role="log" aria-relevant="additions" aria-live="polite">
          {shouldShowSignInPrompt && (
            <div className="surface" role="status">
              Sign in with the controls above to view and participate in chat.
            </div>
          )}
          {sortedMessages.length === 0 ? (
            <div className="chat-panel__empty surface">
              <p className="muted">No messages yet. Be the first to say hello!</p>
            </div>
          ) : (
            <ul className="chat-thread">
              {groupedMessages.map((group) => (
                <li key={group.id} className="chat-message chat-message--group">
                  {showAvatars ? (
                    group.avatar ? (
                      <Image
                        src={group.avatar}
                        alt=""
                        width={44}
                        height={44}
                        sizes="44px"
                        className="chat-message__avatar"
                      />
                    ) : (
                      <div className="chat-message__avatar chat-message__avatar--placeholder" aria-hidden>
                        {group.userLabel.slice(0, 1).toUpperCase()}
                      </div>
                    )
                  ) : (
                    <span className="sr-only">Messages from {group.userLabel}</span>
                  )}
                  <div className="chat-message__content">
                    <div className="chat-message__meta">
                      <div className="chat-message__author">
                        <strong>{group.userLabel}</strong>
                        {group.role && <span className="badge">{group.role}</span>}
                      </div>
                      <span className="muted">{group.messages.length} message{group.messages.length === 1 ? "" : "s"}</span>
                    </div>
                    <div className="chat-message__bubble">
                      {group.messages.map((message) => (
                        <p key={message.id}>
                          {showTimestamps && (
                            <time dateTime={message.sentAt} className="chat-message__time">
                              {new Date(message.sentAt).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}
                            </time>
                          )}
                          {message.message}
                        </p>
                      ))}
                    </div>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
      <form
        className="chat-panel__form"
        onSubmit={handleSend}
        aria-label="Send a chat message"
        aria-disabled={isComposerDisabled}
      >
        <label htmlFor="chat-input" className="sr-only">
          Chat message
        </label>
        <textarea
          id="chat-input"
          name="message"
          placeholder={user ? "Share your thoughts" : "Sign in to participate in chat"}
          value={content}
          onChange={(event) => setContent(event.target.value)}
          disabled={isComposerDisabled}
          rows={3}
          aria-disabled={!user}
        />
        <div className="chat-panel__toolbar">
          <button
            type="submit"
            className="primary-button"
            disabled={isComposerDisabled || content.trim().length === 0}
          >
            {sending ? "Sending…" : "Send"}
          </button>
        </div>
      </form>
      {showPopoutDialog && (
        <div
          className="chat-panel__dialog-backdrop"
          role="presentation"
          onClick={() => setShowPopoutDialog(false)}
        >
          <section
            className="chat-panel__dialog surface"
            role="dialog"
            aria-modal="true"
            aria-labelledby="chat-popout-title"
            onClick={(event) => event.stopPropagation()}
          >
            <header className="chat-panel__dialog-header">
              <h4 id="chat-popout-title">Pop out chat</h4>
              <button
                type="button"
                className="icon-button"
                onClick={() => setShowPopoutDialog(false)}
                aria-label="Close pop-out chat dialog"
              >
                ✕
              </button>
            </header>
            <p className="muted">
              Open the chat in a separate window so you can keep up with the conversation while browsing elsewhere.
            </p>
            <div className="chat-panel__dialog-actions">
              <button type="button" className="ghost-button" onClick={() => setShowPopoutDialog(false)}>
                Cancel
              </button>
              <button type="button" className="accent-button" onClick={openPopoutWindow} aria-label="Open chat in new window">
                Open window
              </button>
            </div>
          </section>
        </div>
      )}
      {showSettings && (
        <div
          className="chat-panel__dialog-backdrop"
          role="presentation"
          onClick={() => setShowSettings(false)}
        >
          <section
            className="chat-panel__dialog surface"
            role="dialog"
            aria-modal="true"
            aria-labelledby="chat-settings-title"
            onClick={(event) => event.stopPropagation()}
          >
            <header className="chat-panel__dialog-header">
              <h4 id="chat-settings-title">Chat settings</h4>
              <button
                type="button"
                className="icon-button"
                onClick={() => setShowSettings(false)}
                aria-label="Close chat settings"
              >
                ✕
              </button>
            </header>
            <div className="chat-panel__settings stack">
              <label className="chat-panel__setting">
                <div className="chat-panel__setting-text">
                  <span>Show avatars</span>
                  <p className="muted">Display profile photos next to each chat participant.</p>
                </div>
                <input
                  type="checkbox"
                  checked={showAvatars}
                  onChange={(event) => setShowAvatars(event.target.checked)}
                  aria-label="Toggle chat avatars"
                />
              </label>
              <label className="chat-panel__setting">
                <div className="chat-panel__setting-text">
                  <span>Show timestamps</span>
                  <p className="muted">Keep message times visible inside the conversation bubbles.</p>
                </div>
                <input
                  type="checkbox"
                  checked={showTimestamps}
                  onChange={(event) => setShowTimestamps(event.target.checked)}
                  aria-label="Toggle chat message timestamps"
                />
              </label>
            </div>
            <div className="chat-panel__dialog-actions">
              <button
                type="button"
                className="ghost-button"
                onClick={() => {
                  setShowAvatars(true);
                  setShowTimestamps(true);
                }}
              >
                Reset
              </button>
              <button type="button" className="accent-button" onClick={() => setShowSettings(false)} aria-label="Save chat settings">
                Done
              </button>
            </div>
          </section>
        </div>
      )}
    </section>
  );
}
