"use client";

import { FormEvent, useEffect, useMemo, useState } from "react";
import { useAuth } from "../hooks/useAuth";
import type { ChatMessage } from "../lib/viewer-api";
import { fetchChannelChat, sendChatMessage } from "../lib/viewer-api";

const POLL_INTERVAL_MS = 10_000;

export function ChatPanel({
  channelId,
  roomId
}: {
  channelId: string;
  roomId?: string;
}) {
  const { user } = useAuth();
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | undefined>();
  const [content, setContent] = useState("");
  const [sending, setSending] = useState(false);
  const [pausedForAuth, setPausedForAuth] = useState(false);
  const chatDisabled = !roomId?.trim();

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

  const sortedMessages = useMemo(
    () =>
      [...messages].sort((a, b) =>
        new Date(a.sentAt).getTime() - new Date(b.sentAt).getTime()
      ),
    [messages]
  );

  useEffect(() => {
    if (chatDisabled) {
      setLoading(false);
      setMessages([]);
      return;
    }

    if (pausedForAuth) {
      if (user) {
        setPausedForAuth(false);
      } else {
        setLoading(false);
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
        }
      } catch (err) {
        if (!cancelled) {
          if (isUnauthorizedError(err) && !user) {
            shouldPoll = false;
            setPausedForAuth(true);
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
  }, [channelId, roomId, user?.id, pausedForAuth]);

  const handleSend = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!content.trim()) {
      return;
    }
    if (!user || chatDisabled) {
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

  const isComposerDisabled = chatDisabled || !user || sending;

  return (
    <section className="chat-panel" aria-live="polite">
      <header className="chat-panel__header">
        <h3>Live chat</h3>
        <span className="muted">{messages.length} message{messages.length === 1 ? "" : "s"}</span>
      </header>
      {chatDisabled && <div className="surface">Chat is disabled/offline</div>}
      {loading && !chatDisabled && <div className="surface">Loading chat…</div>}
      {error && !chatDisabled && <div className="surface" role="alert">{error}</div>}
      {!loading && !error && !chatDisabled && (
        <div className="chat-panel__body" role="log" aria-relevant="additions" aria-live="polite">
          {sortedMessages.length === 0 ? (
            <p className="muted">No messages yet. Be the first to say hello!</p>
          ) : (
            <ul>
              {sortedMessages.map((message) => (
                <li key={message.id} className="chat-message">
                  {message.user?.avatarUrl && (
                    <img
                      src={message.user.avatarUrl}
                      alt=""
                      className="chat-message__avatar"
                      loading="lazy"
                    />
                  )}
                  <div className="chat-message__content">
                    <div className="chat-message__meta">
                      <strong>{message.user?.displayName ?? message.user?.id ?? "Anonymous"}</strong>
                      {message.user?.role && <span className="badge">{message.user.role}</span>}
                      <time dateTime={message.sentAt}>
                        {new Date(message.sentAt).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}
                      </time>
                    </div>
                    <p>{message.message}</p>
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
        aria-disabled={chatDisabled}
      >
        <label htmlFor="chat-input" className="sr-only">
          Chat message
        </label>
        <textarea
          id="chat-input"
          name="message"
          placeholder={
            chatDisabled
              ? "Chat is disabled/offline"
              : user
              ? "Share your thoughts"
              : "Sign in to participate in chat"
          }
          value={content}
          onChange={(event) => setContent(event.target.value)}
          disabled={isComposerDisabled}
          rows={3}
          aria-disabled={chatDisabled || !user}
        />
        <button
          type="submit"
          className="primary-button"
          disabled={isComposerDisabled || content.trim().length === 0}
        >
          {sending ? "Sending…" : "Send"}
        </button>
      </form>
    </section>
  );
}
