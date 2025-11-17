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
          <button type="button" className="icon-button" aria-label="Pop out chat">
            <svg viewBox="0 0 20 20" aria-hidden="true" focusable="false">
              <path
                fill="currentColor"
                d="M4 4h6v2H6v8h8v-4h2v6H4V4Zm6-1.5H18v8h-2V5.91l-4.4 4.4-1.42-1.42L14.59 4H10V2.5Z"
              />
            </svg>
          </button>
          <button type="button" className="icon-button" aria-label="Chat settings">
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
                  {group.avatar ? (
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
                          <time dateTime={message.sentAt} className="chat-message__time">
                            {new Date(message.sentAt).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}
                          </time>
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
          <div className="chat-panel__toolbar-actions">
            <button type="button" className="icon-button" aria-label="Open emotes" disabled={!user}>
              <svg viewBox="0 0 20 20" aria-hidden="true" focusable="false">
                <path
                  fill="currentColor"
                  d="M10 2a8 8 0 1 1 0 16 8 8 0 0 1 0-16Zm0 2a6 6 0 1 0 0 12A6 6 0 0 0 10 4Zm2.5 5a1.5 1.5 0 1 1 0 3 1.5 1.5 0 0 1 0-3Zm-5 0a1.5 1.5 0 1 1 0 3 1.5 1.5 0 0 1 0-3ZM10 14c1.5 0 2.8-.8 3.5-2h-7c.7 1.2 2 2 3.5 2Z"
                />
              </svg>
            </button>
            <button type="button" className="icon-button" aria-label="Attach a file" disabled={!user}>
              <svg viewBox="0 0 20 20" aria-hidden="true" focusable="false">
                <path
                  fill="currentColor"
                  d="M14.5 3.5a2.5 2.5 0 0 1 0 3.54l-6.9 6.9a1.75 1.75 0 0 1-2.47-2.47l6.18-6.18 1.06 1.06-6.18 6.18a.25.25 0 0 0 .35.35l6.9-6.9a1 1 0 0 0-1.42-1.42L8 10.59 6.94 9.53l3.92-3.92a2.5 2.5 0 0 1 3.64 0Z"
                />
              </svg>
            </button>
          </div>
          <button
            type="submit"
            className="primary-button"
            disabled={isComposerDisabled || content.trim().length === 0}
          >
            {sending ? "Sending…" : "Send"}
          </button>
        </div>
      </form>
    </section>
  );
}
