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

  const sortedMessages = useMemo(
    () =>
      [...messages].sort((a, b) =>
        new Date(a.sentAt).getTime() - new Date(b.sentAt).getTime()
      ),
    [messages]
  );

  useEffect(() => {
    if (!roomId) {
      setLoading(false);
      return;
    }

    let cancelled = false;

    const load = async (showSpinner: boolean) => {
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
          setError(err instanceof Error ? err.message : "Unable to load chat");
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    };

    void load(true);
    const interval = setInterval(() => {
      void load(false);
    }, POLL_INTERVAL_MS);

    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, [channelId, roomId]);

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

  return (
    <section className="chat-panel" aria-live="polite">
      <header className="chat-panel__header">
        <h3>Live chat</h3>
        <span className="muted">{messages.length} message{messages.length === 1 ? "" : "s"}</span>
      </header>
      {loading && <div className="surface">Loading chat…</div>}
      {error && <div className="surface" role="alert">{error}</div>}
      {!loading && !error && (
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
      <form className="chat-panel__form" onSubmit={handleSend} aria-label="Send a chat message">
        <label htmlFor="chat-input" className="sr-only">
          Chat message
        </label>
        <textarea
          id="chat-input"
          name="message"
          placeholder={user ? "Share your thoughts" : "Sign in to participate in chat"}
          value={content}
          onChange={(event) => setContent(event.target.value)}
          disabled={!user || sending}
          rows={3}
          aria-disabled={!user}
        />
        <button type="submit" className="primary-button" disabled={!user || sending || content.trim().length === 0}>
          {sending ? "Sending…" : "Send"}
        </button>
      </form>
    </section>
  );
}
