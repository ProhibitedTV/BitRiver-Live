"use client";

import Image from "next/image";
import { FormEvent, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { useAuth } from "../hooks/useAuth";
import type { ChatMessage } from "../lib/viewer-api";
import { fetchChannelChat, reportChatMessage, sendChatMessage, viewerWebSocketUrl } from "../lib/viewer-api";

const POLL_INTERVAL_MS = 10_000;
const MAX_RETRY_DELAY_MS = 60_000;
const MAX_CONSECUTIVE_FAILURES = 5;
const MAX_MESSAGES = 500;
const RETRY_MESSAGE = "Unable to load chat. We'll retry in a bit.";
const CHAT_SOCKET_PATH = "/api/chat/ws";

type ChatMessageEntry = {
  message: ChatMessage;
  sentAtTs: number;
};

type ChatRoomNotice = {
  id: string;
  tone: "system" | "moderation" | "error";
  label: string;
  message: string;
  occurredAt: string;
  occurredAtTs: number;
};

type ChatTranscriptEntry =
  | { type: "message"; entry: ChatMessageEntry }
  | { type: "notice"; notice: ChatRoomNotice };

type ChatGatewayEnvelope = {
  type?: string;
  error?: string;
  event?: {
    type?: string;
    occurredAt?: string;
    targetId?: string;
    actorId?: string;
    reason?: string;
    moderation?: {
      action?: string;
      actorId?: string;
      targetId?: string;
      reason?: string;
      expiresAt?: string;
    };
    report?: {
      reporterId?: string;
      targetId?: string;
      reason?: string;
      status?: string;
      messageId?: string;
    };
    automod?: {
      userId?: string;
      filterKind?: string;
      filterPattern?: string;
      action?: string;
      message?: string;
    };
    system?: {
      kind?: string;
      message?: string;
      actorId?: string;
      targetId?: string;
    };
    message?: {
      id?: string;
      channelId?: string;
      userId?: string;
      user?: {
        id?: string;
        displayName?: string;
        role?: string;
        badges?: { id?: string; label?: string }[];
      };
      content?: string;
      createdAt?: string;
    };
    filter?: {
      name?: string;
      action?: string;
    };
  };
};

type ChatAuthUser = {
  id: string;
  displayName: string;
};

type ChatPanelProps = {
  channelId: string;
  channelOwnerId?: string;
  roomId?: string;
  roomName?: string;
  live?: boolean;
  viewerCount?: number;
};

type ChatModerationCommand = {
  type: "timeout" | "remove_timeout" | "ban" | "unban";
  targetId: string;
  durationMs?: number;
  reason?: string;
};

type ChatCommandResult =
  | { kind: "message"; content: string }
  | { kind: "clear" }
  | { kind: "moderation"; command: ChatModerationCommand; targetLabel: string }
  | { kind: "error"; message: string };

const MESSAGE_ROW_TIMEOUT_MS = 5 * 60 * 1000;
const MODERATOR_ROLES = new Set(["admin", "moderator"]);

function canModerateRoom(user: { id: string; roles?: string[] } | undefined, channelOwnerId?: string) {
  if (!user) {
    return false;
  }
  if (channelOwnerId && user.id === channelOwnerId) {
    return true;
  }
  return (user.roles ?? []).some((role) => MODERATOR_ROLES.has(role));
}

function parseDurationMs(input: string): number | undefined {
  const match = input.trim().match(/^(\d+)(ms|s|m|h|d)?$/i);
  if (!match) {
    return undefined;
  }
  const value = Number(match[1]);
  if (!Number.isSafeInteger(value) || value <= 0) {
    return undefined;
  }
  const unit = (match[2] ?? "s").toLowerCase();
  const multiplier =
    unit === "ms"
      ? 1
      : unit === "s"
        ? 1000
        : unit === "m"
          ? 60 * 1000
          : unit === "h"
            ? 60 * 60 * 1000
            : 24 * 60 * 60 * 1000;
  const durationMs = value * multiplier;
  return Number.isSafeInteger(durationMs) ? durationMs : undefined;
}

function normalizeTargetToken(input: string) {
  return input.trim().replace(/^@/, "").toLowerCase();
}

function resolveCommandTarget(input: string, messages: ChatMessageEntry[]) {
  const normalized = normalizeTargetToken(input);
  if (!normalized) {
    return "";
  }
  const users = messages
    .map((entry) => entry.message.user)
    .filter((candidate): candidate is NonNullable<ChatMessage["user"]> => Boolean(candidate?.id));
  const byId = users.find((candidate) => candidate.id.toLowerCase() === normalized);
  if (byId) {
    return byId.id;
  }
  const byDisplayName = users.find((candidate) => candidate.displayName.toLowerCase() === normalized);
  if (byDisplayName) {
    return byDisplayName.id;
  }
  return input.trim().replace(/^@/, "");
}

function parseChatCommand(input: string, messages: ChatMessageEntry[]): ChatCommandResult {
  const trimmed = input.trim();
  if (!trimmed.startsWith("/")) {
    return { kind: "message", content: trimmed };
  }

  const [rawCommand, ...args] = trimmed.slice(1).split(/\s+/);
  const command = rawCommand.toLowerCase();

  if (command === "clear") {
    return { kind: "clear" };
  }
  if (command === "me") {
    return { kind: "error", message: "Action messages are not supported yet." };
  }
  if (command === "timeout") {
    const [target, duration, ...reasonParts] = args;
    if (!target || !duration) {
      return { kind: "error", message: "Usage: /timeout <user> <duration> [reason]" };
    }
    const durationMs = parseDurationMs(duration);
    if (!durationMs) {
      return { kind: "error", message: "Use a positive timeout duration such as 30s, 5m, or 1h." };
    }
    return {
      kind: "moderation",
      targetLabel: target,
      command: {
        type: "timeout",
        targetId: resolveCommandTarget(target, messages),
        durationMs,
        reason: reasonParts.join(" ").trim() || undefined,
      },
    };
  }
  if (command === "ban") {
    const [target, ...reasonParts] = args;
    if (!target) {
      return { kind: "error", message: "Usage: /ban <user> [reason]" };
    }
    return {
      kind: "moderation",
      targetLabel: target,
      command: {
        type: "ban",
        targetId: resolveCommandTarget(target, messages),
        reason: reasonParts.join(" ").trim() || undefined,
      },
    };
  }
  if (command === "unban" || command === "remove_timeout" || command === "untimeout") {
    const [target] = args;
    if (!target) {
      return { kind: "error", message: `Usage: /${command} <user>` };
    }
    return {
      kind: "moderation",
      targetLabel: target,
      command: {
        type: command === "unban" ? "unban" : "remove_timeout",
        targetId: resolveCommandTarget(target, messages),
      },
    };
  }
  return {
    kind: "error",
    message: "Unknown chat command. Try /timeout, /ban, /unban, /remove_timeout, or /clear.",
  };
}

function chatMessageFromGatewayEnvelope(envelope: ChatGatewayEnvelope, currentUser?: ChatAuthUser): ChatMessage | undefined {
  const event = envelope.event;
  const message = event?.message;
  if (event?.type !== "message" || !message?.id || !message.content) {
    return undefined;
  }

  const userId = message.userId?.trim() ?? "";
  const displayName = message.user?.displayName?.trim();
  const role = message.user?.role?.trim();
  const badges = message.user?.badges?.reduce<{ id: string; label?: string }[]>((items, badge) => {
    const id = badge.id?.trim();
    if (!id) {
      return items;
    }
    const label = badge.label?.trim();
    items.push(label ? { id, label } : { id });
    return items;
  }, []);
  return {
    id: message.id,
    message: message.content,
    sentAt: message.createdAt ?? event.occurredAt ?? new Date().toISOString(),
    user: userId
      ? {
          id: userId,
          displayName: displayName || (currentUser?.id === userId ? currentUser.displayName : userId),
          role: role || undefined,
          badges: badges && badges.length > 0 ? badges : undefined,
        }
      : undefined,
  };
}

function formatNoticeEventType(type: string) {
  return type
    .split(/[_-]/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ");
}

function chatNoticeFromGatewayEnvelope(envelope: ChatGatewayEnvelope): ChatRoomNotice | undefined {
  if (envelope.type === "error") {
    const now = new Date().toISOString();
    return {
      id: `error-${now}-${envelope.error ?? "chat"}`,
      tone: "error",
      label: "Chat error",
      message: envelope.error || "Live chat connection error",
      occurredAt: now,
      occurredAtTs: new Date(now).getTime(),
    };
  }

  const event = envelope.event;
  if (envelope.type !== "event" || !event?.type || event.type === "message") {
    return undefined;
  }

  const eventType = event.type;
  const occurredAt = event.occurredAt ?? new Date().toISOString();
  const moderation = event.moderation;
  const report = event.report;
  const automod = event.automod;
  const system = event.system;
  const moderationTypes = new Set(["automod", "moderation", "timeout", "ban", "unban", "remove_timeout"]);
  const label =
    eventType === "automod"
      ? "Automod"
      : eventType === "moderation" && moderation?.action
        ? formatNoticeEventType(moderation.action)
        : system?.kind
          ? formatNoticeEventType(system.kind)
          : formatNoticeEventType(eventType);
  const targetId = event.targetId ?? moderation?.targetId ?? report?.targetId ?? system?.targetId ?? automod?.userId;
  const reason = event.reason ?? moderation?.reason ?? report?.reason;
  const filterAction = event.filter?.action ?? automod?.action;
  const detailParts = [
    system?.message,
    event.filter?.name ? `Filter: ${event.filter.name}` : undefined,
    filterAction ? `Action: ${filterAction}` : undefined,
    automod?.filterKind ? `Filter kind: ${automod.filterKind}` : undefined,
    automod?.filterPattern ? `Filter: ${automod.filterPattern}` : undefined,
    targetId ? `Target: ${targetId}` : undefined,
    reason ? `Reason: ${reason}` : undefined,
    report?.status ? `Status: ${report.status}` : undefined,
    moderation?.expiresAt ? `Expires: ${new Date(moderation.expiresAt).toLocaleString()}` : undefined,
  ].filter(Boolean);

  return {
    id: `${eventType}-${occurredAt}-${targetId ?? event.actorId ?? moderation?.actorId ?? report?.reporterId ?? detailParts.join("-")}`,
    tone: moderationTypes.has(eventType) ? "moderation" : "system",
    label,
    message: detailParts.length > 0 ? detailParts.join(" - ") : `${label} event received.`,
    occurredAt,
    occurredAtTs: new Date(occurredAt).getTime(),
  };
}

export function ChatPanel({
  channelId,
  channelOwnerId,
  roomId,
  roomName,
  live,
  viewerCount
}: ChatPanelProps) {
  const { user, loading: authLoading, signIn } = useAuth();
  const [messageEntries, setMessageEntries] = useState<ChatMessageEntry[]>([]);
  const [noticeEntries, setNoticeEntries] = useState<ChatRoomNotice[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | undefined>();
  const [content, setContent] = useState("");
  const [sending, setSending] = useState(false);
  const [pausedForAuth, setPausedForAuth] = useState(false);
  const [authRequired, setAuthRequired] = useState(false);
  const [optionsOpen, setOptionsOpen] = useState(false);
  const [showAvatars, setShowAvatars] = useState(true);
  const [showTimestamps, setShowTimestamps] = useState(true);
  const [reportingMessageId, setReportingMessageId] = useState<string | undefined>();
  const [reportReason, setReportReason] = useState("");
  const [reportSubmitting, setReportSubmitting] = useState(false);
  const [reportError, setReportError] = useState<string | undefined>();
  const [reportNotice, setReportNotice] = useState<string | undefined>();
  const [moderationError, setModerationError] = useState<string | undefined>();
  const [moderationNotice, setModerationNotice] = useState<string | undefined>();
  const optionsTriggerRef = useRef<HTMLButtonElement | null>(null);
  const optionsMenuRef = useRef<HTMLDivElement | null>(null);
  const reportDialogRef = useRef<HTMLElement | null>(null);
  const reportHeadingRef = useRef<HTMLHeadingElement | null>(null);
  const socketRef = useRef<WebSocket | null>(null);
  const transcriptScrollRef = useRef<HTMLDivElement | null>(null);
  const shouldFollowTranscriptRef = useRef(true);
  const [socketReady, setSocketReady] = useState(false);
  const [newMessagesWaiting, setNewMessagesWaiting] = useState(false);
  const canModerate = canModerateRoom(user, channelOwnerId);

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

  const applyMessages = (incoming: ChatMessage[] | ChatMessage) => {
    setMessageEntries((prev) => {
      const normalize = (message: ChatMessage): ChatMessageEntry => ({
        message,
        sentAtTs: new Date(message.sentAt).getTime()
      });
      const isMonotonic = (entries: ChatMessageEntry[]) => {
        for (let index = 1; index < entries.length; index += 1) {
          if (entries[index].sentAtTs < entries[index - 1].sentAtTs) {
            return false;
          }
        }
        return true;
      };

      const next = Array.isArray(incoming)
        ? incoming.map(normalize)
        : prev.some((entry) => entry.message.id === incoming.id)
          ? prev
          : [...prev, normalize(incoming)];
      const truncated =
        next.length <= MAX_MESSAGES ? next : next.slice(next.length - MAX_MESSAGES);
      if (truncated.length < 2) {
        return truncated;
      }

      const alreadyOrdered = Array.isArray(incoming)
        ? isMonotonic(truncated)
        : prev.length === 0 || prev[prev.length - 1].sentAtTs <= truncated[truncated.length - 1].sentAtTs;
      if (alreadyOrdered) {
        return truncated;
      }
      return [...truncated].sort((a, b) => a.sentAtTs - b.sentAtTs);
    });
  };

  const appendNotice = (notice: ChatRoomNotice) => {
    setNoticeEntries((prev) => {
      if (prev.some((entry) => entry.id === notice.id)) {
        return prev;
      }
      const next = [...prev, notice];
      return next.length <= 80 ? next : next.slice(next.length - 80);
    });
  };

  const handleTranscriptScroll = () => {
    const scrollElement = transcriptScrollRef.current;
    if (!scrollElement) {
      return;
    }
    const distanceFromBottom =
      scrollElement.scrollHeight - scrollElement.scrollTop - scrollElement.clientHeight;
    const isNearBottom = distanceFromBottom <= 64;
    shouldFollowTranscriptRef.current = isNearBottom;
    if (isNearBottom) {
      setNewMessagesWaiting(false);
    }
  };

  const scrollTranscriptToBottom = () => {
    const scrollElement = transcriptScrollRef.current;
    if (!scrollElement) {
      return;
    }
    shouldFollowTranscriptRef.current = true;
    scrollElement.scrollTop = scrollElement.scrollHeight;
    setNewMessagesWaiting(false);
  };

  useEffect(() => {
    if (!user || typeof window === "undefined" || typeof WebSocket === "undefined") {
      setSocketReady(false);
      socketRef.current = null;
      return undefined;
    }

    let cancelled = false;
    let socket: WebSocket;
    try {
      socket = new WebSocket(viewerWebSocketUrl(CHAT_SOCKET_PATH));
    } catch {
      setSocketReady(false);
      socketRef.current = null;
      return undefined;
    }

    socketRef.current = socket;
    setSocketReady(false);

    const handleOpen = () => {
      if (cancelled) {
        return;
      }
      socket.send(JSON.stringify({ type: "join", channelId }));
      setSocketReady(true);
      setAuthRequired(false);
    };

    const handleMessage = (event: MessageEvent) => {
      if (cancelled || typeof event.data !== "string") {
        return;
      }
      let envelope: ChatGatewayEnvelope;
      try {
        envelope = JSON.parse(event.data) as ChatGatewayEnvelope;
      } catch {
        return;
      }
      if (envelope.type === "error") {
        setError(envelope.error || "Live chat connection error");
        appendNotice(
          chatNoticeFromGatewayEnvelope(envelope) ?? {
            id: `error-${Date.now()}`,
            tone: "error",
            label: "Chat error",
            message: envelope.error || "Live chat connection error",
            occurredAt: new Date().toISOString(),
            occurredAtTs: Date.now(),
          }
        );
        return;
      }
      const chatMessage = chatMessageFromGatewayEnvelope(envelope, user);
      if (chatMessage) {
        applyMessages(chatMessage);
        setAuthRequired(false);
        return;
      }
      const notice = chatNoticeFromGatewayEnvelope(envelope);
      if (notice) {
        appendNotice(notice);
      }
    };

    const handleClose = () => {
      if (socketRef.current === socket) {
        socketRef.current = null;
      }
      if (!cancelled) {
        setSocketReady(false);
      }
    };

    const handleError = () => {
      if (!cancelled) {
        setSocketReady(false);
      }
    };

    socket.addEventListener("open", handleOpen);
    socket.addEventListener("message", handleMessage);
    socket.addEventListener("close", handleClose);
    socket.addEventListener("error", handleError);

    return () => {
      cancelled = true;
      socket.removeEventListener("open", handleOpen);
      socket.removeEventListener("message", handleMessage);
      socket.removeEventListener("close", handleClose);
      socket.removeEventListener("error", handleError);
      if (socketRef.current === socket) {
        socketRef.current = null;
      }
      setSocketReady(false);
      if (socket.readyState === 0 || socket.readyState === 1) {
        socket.close();
      }
    };
  }, [channelId, user]);

  const participantPreview = useMemo(() => {
    const participants = new Map<string, NonNullable<ChatMessage["user"]>>();
    if (user) {
      participants.set(user.id, { id: user.id, displayName: user.displayName, role: "you" });
    }
    messageEntries.forEach(({ message }) => {
      if (message.user?.id && !participants.has(message.user.id)) {
        participants.set(message.user.id, message.user);
      }
    });
    return Array.from(participants.values()).slice(0, 5);
  }, [messageEntries, user]);

  const groupedMessages = useMemo(() => {
    const groups: {
      id: string;
      userLabel: string;
      avatar?: string;
      role?: string;
      messages: { message: ChatMessage; sentAtTs: number }[];
    }[] = [];
    const TIME_DELTA_MS = 2 * 60 * 1000;
    messageEntries.forEach((entry) => {
      const { message, sentAtTs } = entry;
      const displayName =
        message.user?.displayName ?? message.user?.id ?? "Anonymous";
      const previous = groups[groups.length - 1];
      const previousDate = previous?.messages.length
        ? previous.messages[previous.messages.length - 1].sentAtTs
        : undefined;
      const sameUser = previous?.userLabel === displayName;
      const withinWindow = previousDate
        ? Math.abs(sentAtTs - previousDate) <= TIME_DELTA_MS
        : false;

      if (previous && sameUser && withinWindow) {
        previous.messages.push(entry);
      } else {
        groups.push({
          id: message.id,
          userLabel: displayName,
          avatar: message.user?.avatarUrl,
          role: message.user?.role,
          messages: [entry]
        });
      }
    });
    return groups;
  }, [messageEntries]);

  const groupedMessagesByFirstId = useMemo(() => {
    const groupsById = new Map<string, (typeof groupedMessages)[number]>();
    groupedMessages.forEach((group) => {
      groupsById.set(group.id, group);
    });
    return groupsById;
  }, [groupedMessages]);

  const transcriptEntries = useMemo<ChatTranscriptEntry[]>(() => {
    const entries: ChatTranscriptEntry[] = [
      ...messageEntries.map((entry) => ({ type: "message" as const, entry })),
      ...noticeEntries.map((notice) => ({ type: "notice" as const, notice })),
    ];
    return entries.sort((left, right) => {
      const leftTime = left.type === "message" ? left.entry.sentAtTs : left.notice.occurredAtTs;
      const rightTime = right.type === "message" ? right.entry.sentAtTs : right.notice.occurredAtTs;
      return leftTime - rightTime;
    });
  }, [messageEntries, noticeEntries]);

  useLayoutEffect(() => {
    const scrollElement = transcriptScrollRef.current;
    if (!scrollElement || transcriptEntries.length === 0) {
      return;
    }
    if (shouldFollowTranscriptRef.current) {
      scrollElement.scrollTop = scrollElement.scrollHeight;
      setNewMessagesWaiting(false);
      return;
    }
    setNewMessagesWaiting(true);
  }, [transcriptEntries.length]);

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
    let timeout: ReturnType<typeof setTimeout> | undefined;
    let consecutiveFailures = 0;

    const scheduleNextPoll = (delay: number) => {
      if (cancelled || !shouldPoll) {
        return;
      }
      timeout = setTimeout(() => {
        void load(false);
      }, delay);
    };

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
          applyMessages(chatMessages);
          setAuthRequired(false);
          consecutiveFailures = 0;
          scheduleNextPoll(POLL_INTERVAL_MS);
        }
      } catch (err) {
        if (!cancelled) {
          if (isUnauthorizedError(err) && !user) {
            shouldPoll = false;
            setPausedForAuth(true);
            setAuthRequired(true);
            setMessageEntries([]);
            setError(undefined);
          } else {
            consecutiveFailures += 1;
            setError((previous) => previous ?? RETRY_MESSAGE);
            const backoffDelay = Math.min(
              POLL_INTERVAL_MS * 2 ** consecutiveFailures,
              MAX_RETRY_DELAY_MS
            );
            const nextDelay =
              consecutiveFailures >= MAX_CONSECUTIVE_FAILURES
                ? MAX_RETRY_DELAY_MS
                : backoffDelay;
            scheduleNextPoll(nextDelay);
          }
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    };

    void load(true);

    return () => {
      cancelled = true;
      if (timeout) {
        clearTimeout(timeout);
      }
    };
  }, [channelId, roomId, user, pausedForAuth]);

  const sendModerationCommand = (command: ChatModerationCommand, targetLabel: string) => {
    if (!user) {
      setModerationError("Sign in to moderate chat.");
      setModerationNotice(undefined);
      return false;
    }
    if (!canModerate) {
      setModerationError("You do not have permission to moderate this room.");
      setModerationNotice(undefined);
      return false;
    }
    const socket = socketRef.current;
    if (!socketReady || socket?.readyState !== 1) {
      setModerationError("Moderation commands require the live chat connection.");
      setModerationNotice(undefined);
      return false;
    }
    socket.send(JSON.stringify({ ...command, channelId }));
    setModerationError(undefined);
    setModerationNotice(`${formatNoticeEventType(command.type)} command sent for ${targetLabel}.`);
    return true;
  };

  const handleModerationAction = (command: ChatModerationCommand, targetLabel: string) => {
    if (
      command.type === "ban" &&
      typeof window !== "undefined" &&
      !window.confirm(`Ban ${targetLabel} from chat?`)
    ) {
      return;
    }
    sendModerationCommand(command, targetLabel);
  };

  const handleSend = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const trimmedContent = content.trim();
    if (!trimmedContent) {
      return;
    }
    if (!user) {
      return;
    }

    const parsedCommand = parseChatCommand(trimmedContent, messageEntries);
    if (parsedCommand.kind === "error") {
      setModerationError(parsedCommand.message);
      setModerationNotice(undefined);
      return;
    }
    if (parsedCommand.kind === "clear") {
      setMessageEntries([]);
      setNoticeEntries([]);
      setNewMessagesWaiting(false);
      setContent("");
      setModerationError(undefined);
      setModerationNotice("Local chat view cleared.");
      return;
    }
    if (parsedCommand.kind === "moderation") {
      if (sendModerationCommand(parsedCommand.command, parsedCommand.targetLabel)) {
        setContent("");
      }
      return;
    }

    try {
      setSending(true);
      setModerationError(undefined);
      const socket = socketRef.current;
      if (socketReady && socket?.readyState === 1) {
        socket.send(JSON.stringify({ type: "message", channelId, content: parsedCommand.content }));
        setContent("");
        return;
      }
      const message = await sendChatMessage(
        channelId,
        user.id,
        parsedCommand.content
      );
      applyMessages(message);
      setContent("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to send message");
    } finally {
      setSending(false);
    }
  };

  const openReportForm = (message: ChatMessage) => {
    setReportingMessageId(message.id);
    setReportReason("");
    setReportError(undefined);
    setReportNotice(undefined);
  };

  const closeReportForm = () => {
    setReportingMessageId(undefined);
    setReportReason("");
    setReportError(undefined);
  };

  const handleReportSubmit = async (event: FormEvent<HTMLFormElement>, message: ChatMessage) => {
    event.preventDefault();
    if (!user) {
      setReportError("Sign in to report chat messages.");
      return;
    }
    const targetId = message.user?.id?.trim();
    if (!targetId || targetId === user.id) {
      setReportError("This message cannot be reported from your account.");
      return;
    }
    const reason = reportReason.trim();
    if (!reason) {
      setReportError("Add a reason before submitting this report.");
      return;
    }

    try {
      setReportSubmitting(true);
      setReportError(undefined);
      await reportChatMessage(channelId, {
        targetId,
        messageId: message.id,
        reason,
      });
      setReportNotice("Report submitted for moderator review.");
      setReportingMessageId(undefined);
      setReportReason("");
    } catch (err) {
      setReportError(err instanceof Error ? err.message : "Unable to submit report");
    } finally {
      setReportSubmitting(false);
    }
  };

  const isComposerDisabled = !user || sending;

  const shouldShowSignInPrompt = authRequired && !authLoading && !user;

  const openPopoutWindow = () => {
    if (typeof window === "undefined") {
      return;
    }
    window.open(
      window.location.href,
      "bitriver-chat-popout",
      "width=420,height=720,noopener,noreferrer"
    );
    setOptionsOpen(false);
    optionsTriggerRef.current?.focus({ preventScroll: true });
  };

  const selectedReportMessage = useMemo(() => {
    if (!reportingMessageId) {
      return undefined;
    }
    return messageEntries.find((entry) => entry.message.id === reportingMessageId)?.message;
  }, [messageEntries, reportingMessageId]);

  useEffect(() => {
    if (!optionsOpen || typeof document === "undefined") {
      return;
    }
    const closeOptions = (restoreFocus = true) => {
      setOptionsOpen(false);
      if (restoreFocus) {
        optionsTriggerRef.current?.focus({ preventScroll: true });
      }
    };
    const handleOutsideInteraction = (event: PointerEvent | MouseEvent) => {
      const target = event.target;
      if (!(target instanceof Node)) {
        return;
      }
      if (optionsTriggerRef.current?.contains(target) || optionsMenuRef.current?.contains(target)) {
        return;
      }
      closeOptions(false);
    };
    const handleEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        closeOptions();
      }
    };
    document.addEventListener("pointerdown", handleOutsideInteraction);
    document.addEventListener("click", handleOutsideInteraction);
    document.addEventListener("keydown", handleEscape);
    return () => {
      document.removeEventListener("pointerdown", handleOutsideInteraction);
      document.removeEventListener("click", handleOutsideInteraction);
      document.removeEventListener("keydown", handleEscape);
    };
  }, [optionsOpen]);

  useEffect(() => {
    if (!selectedReportMessage) {
      return;
    }
    reportHeadingRef.current?.focus({ preventScroll: true });

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        closeReportForm();
        return;
      }
      if (event.key !== "Tab") {
        return;
      }
      const activeDialog = reportDialogRef.current;
      if (!activeDialog) {
        return;
      }
      const focusableSelectors =
        'a[href], button:not([disabled]), textarea, input, select, [tabindex]:not([tabindex="-1"])';
      const focusable = Array.from(
        activeDialog.querySelectorAll<HTMLElement>(focusableSelectors)
      ).filter((element) => {
        if (element.hasAttribute("disabled")) {
          return false;
        }
        if (element.getAttribute("aria-hidden") === "true") {
          return false;
        }
        if (element.tabIndex === -1) {
          return false;
        }
        return true;
      });
      if (focusable.length === 0) {
        return;
      }
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      const active = document.activeElement as HTMLElement | null;
      if (!active || !activeDialog.contains(active)) {
        event.preventDefault();
        first.focus();
        return;
      }
      if (event.shiftKey) {
        if (active === first) {
          event.preventDefault();
          last.focus();
        }
        return;
      }
      if (active === last) {
        event.preventDefault();
        first.focus();
      }
    };
    const activeDialog = reportDialogRef.current;
    activeDialog?.addEventListener("keydown", handleKeyDown);
    return () => activeDialog?.removeEventListener("keydown", handleKeyDown);
  }, [selectedReportMessage]);

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

  const roomTitle = roomName?.trim();
  const roomStatusLabel = live === false ? "Offline room" : "Live room";
  const syncLabel = socketReady ? "Live sync" : user ? "Reconnecting" : "Sign in required";

  return (
    <section className="chat-panel">
      <header className="chat-panel__header">
        <div className="chat-panel__title">
          <span className="chat-panel__eyebrow">Live chat</span>
          <h3>Live chat</h3>
          {roomTitle ? <p className="chat-panel__room-name">{roomTitle}</p> : null}
          <div className="chat-panel__counts">
            <span className={`pill ${live === false ? "pill--ghost" : "pill--live"}`}>
              {roomStatusLabel}
            </span>
            {viewerCount !== undefined && (
              <span className="pill pill--ghost">
                {viewerCount.toLocaleString()} viewers
              </span>
            )}
            <span className="pill pill--ghost">
              {messageEntries.length} messages
            </span>
          </div>
        </div>
        <div className="chat-panel__actions" aria-label="Chat actions">
          <button
            type="button"
            className="secondary-button chat-panel__options-trigger"
            aria-label="Open chat options"
            aria-expanded={optionsOpen}
            aria-controls="chat-options-menu"
            onClick={() => setOptionsOpen((open) => !open)}
            ref={optionsTriggerRef}
          >
            ...
          </button>
          <div
            id="chat-options-menu"
            className={`chat-panel__options-menu${optionsOpen ? " chat-panel__options-menu--open" : ""}`}
            ref={optionsMenuRef}
            hidden={!optionsOpen}
            aria-label="Chat options"
          >
            <div className="chat-panel__connection">
              <span className={`chat-panel__connection-dot${socketReady ? " chat-panel__connection-dot--live" : ""}`} aria-hidden="true" />
              <span>{syncLabel}</span>
            </div>
            <button type="button" className="chat-panel__menu-action" onClick={openPopoutWindow}>
              Open pop-out chat
            </button>
            <label className="chat-panel__menu-setting">
              <span>Show avatars</span>
              <input
                type="checkbox"
                checked={showAvatars}
                onChange={(event) => setShowAvatars(event.target.checked)}
                aria-label="Toggle chat avatars"
              />
            </label>
            <label className="chat-panel__menu-setting">
              <span>Show timestamps</span>
              <input
                type="checkbox"
                checked={showTimestamps}
                onChange={(event) => setShowTimestamps(event.target.checked)}
                aria-label="Toggle chat message timestamps"
              />
            </label>
          </div>
        </div>
      </header>
      {/* Gate the thread UI behind loading skeletons, retry errors, and auth-required sign-in prompts. */}
      {loading && renderSkeletons()}
      {error && (
        <div className="surface" role="alert">
          {error}
        </div>
      )}
      {reportNotice && (
        <div className="surface" role="status">
          {reportNotice}
        </div>
      )}
      {reportError && (
        <div className="surface" role="alert">
          {reportError}
        </div>
      )}
      {moderationNotice && (
        <div className="surface" role="status">
          {moderationNotice}
        </div>
      )}
      {moderationError && (
        <div className="surface" role="alert">
          {moderationError}
        </div>
      )}
      {!loading && !error && (!shouldShowSignInPrompt || messageEntries.length > 0) && (
        <div
          className="chat-panel__body"
          ref={transcriptScrollRef}
          onScroll={handleTranscriptScroll}
          data-testid="chat-transcript-scroll"
        >
          <aside className="chat-panel__roster" aria-label="Room roster">
            <div>
              <span className="chat-panel__roster-label">Room</span>
              <strong>
                {viewerCount !== undefined
                  ? `${viewerCount.toLocaleString()} viewers total`
                  : `${participantPreview.length} present`}
              </strong>
            </div>
            {participantPreview.length > 0 ? (
              <ul className="chat-panel__roster-list" aria-label="Recent chatters">
                {participantPreview.map((participant) => (
                  <li key={participant.id}>
                    <span className="chat-panel__roster-avatar" aria-hidden>
                      {participant.displayName.slice(0, 1).toUpperCase()}
                    </span>
                    <span>{participant.displayName}</span>
                    {participant.role ? <span className="chat-panel__role-slot">{participant.role}</span> : null}
                  </li>
                ))}
              </ul>
            ) : (
              <p className="muted">Chatters will appear here as the room wakes up.</p>
            )}
          </aside>
          {transcriptEntries.length === 0 ? (
            <div className="chat-panel__empty surface">
              <p className="muted">
                No messages yet. Be the first to say hello!
              </p>
            </div>
          ) : (
            // Render author/time-window groups with avatar fallbacks and optional per-message timestamps.
            <ul
              className="chat-thread"
              role="log"
              // Screen readers should announce only newly added chat entries as they stream in.
              aria-live="polite"
              aria-relevant="additions text"
              aria-atomic="false"
            >
              {transcriptEntries.map((transcriptEntry) => {
                if (transcriptEntry.type === "notice") {
                  const { notice } = transcriptEntry;
                  return (
                    <li key={notice.id} className={`chat-system-row chat-system-row--${notice.tone}`}>
                      <span className="chat-system-row__label">{notice.label}</span>
                      <p>{notice.message}</p>
                      {showTimestamps ? (
                        <time dateTime={notice.occurredAt}>
                          {new Date(notice.occurredAt).toLocaleTimeString([], {
                            hour: "2-digit",
                            minute: "2-digit"
                          })}
                        </time>
                      ) : null}
                    </li>
                  );
                }

                const group = groupedMessagesByFirstId.get(transcriptEntry.entry.message.id);
                if (!group) {
                  return null;
                }
                return (
                  <li key={group.id} className="chat-message chat-message--group">
                    {showAvatars ? (
                      group.avatar ? (
                        <Image
                          src={group.avatar}
                          alt=""
                          width={36}
                          height={36}
                          sizes="36px"
                          className="chat-message__avatar"
                        />
                      ) : (
                        <div
                          className="chat-message__avatar chat-message__avatar--placeholder"
                          aria-hidden
                        >
                          {group.userLabel.slice(0, 1).toUpperCase()}
                        </div>
                      )
                    ) : (
                      <span className="sr-only">
                        Messages from {group.userLabel}
                      </span>
                    )}
                    <div className="chat-message__content">
                      <div className="chat-message__meta">
                        <div className="chat-message__author">
                          <strong>{group.userLabel}</strong>
                          <span className="chat-panel__badge-slot" aria-hidden />
                          {group.role && <span className="badge">{group.role}</span>}
                        </div>
                        <span className="muted">
                          {group.messages.length} message
                          {group.messages.length === 1 ? "" : "s"}
                        </span>
                      </div>
                      <div className="chat-message__bubble">
                        {group.messages.map(({ message }) => {
                          const targetId = message.user?.id?.trim() ?? "";
                          const targetLabel = message.user?.displayName ?? message.user?.id ?? "chat participant";
                          const canReportMessage = Boolean(user && targetId && targetId !== user.id);
                          const canModerateMessage = Boolean(canModerate && targetId && targetId !== user?.id);
                          return (
                            <div key={message.id} className="chat-message__line">
                              <p>
                                {showTimestamps && (
                                  <time
                                    dateTime={message.sentAt}
                                    className="chat-message__time"
                                  >
                                    {new Date(message.sentAt).toLocaleTimeString([], {
                                      hour: "2-digit",
                                      minute: "2-digit"
                                    })}
                                  </time>
                                )}
                                {message.message}
                              </p>
                              {canReportMessage || canModerateMessage ? (
                                <div className="chat-message__actions" aria-label={`Actions for ${targetLabel}`}>
                                  {canReportMessage ? (
                                    <button
                                      type="button"
                                      className="chat-message__report-button"
                                      onClick={() => openReportForm(message)}
                                      aria-label={`Report message from ${targetLabel}`}
                                    >
                                      Report
                                    </button>
                                  ) : null}
                                  {canModerateMessage ? (
                                    <>
                                      <button
                                        type="button"
                                        className="chat-message__report-button"
                                        onClick={() =>
                                          handleModerationAction(
                                            {
                                              type: "timeout",
                                              targetId,
                                              durationMs: MESSAGE_ROW_TIMEOUT_MS,
                                              reason: "Moderated from viewer chat",
                                            },
                                            targetLabel
                                          )
                                        }
                                        aria-label={`Timeout ${targetLabel} for 5 minutes`}
                                      >
                                        Timeout 5m
                                      </button>
                                      <button
                                        type="button"
                                        className="chat-message__report-button"
                                        onClick={() =>
                                          handleModerationAction(
                                            { type: "remove_timeout", targetId },
                                            targetLabel
                                          )
                                        }
                                        aria-label={`Remove timeout for ${targetLabel}`}
                                      >
                                        Remove timeout
                                      </button>
                                      <button
                                        type="button"
                                        className="chat-message__report-button"
                                        onClick={() =>
                                          handleModerationAction(
                                            { type: "ban", targetId, reason: "Moderated from viewer chat" },
                                            targetLabel
                                          )
                                        }
                                        aria-label={`Ban ${targetLabel}`}
                                      >
                                        Ban
                                      </button>
                                      <button
                                        type="button"
                                        className="chat-message__report-button"
                                        onClick={() =>
                                          handleModerationAction(
                                            { type: "unban", targetId },
                                            targetLabel
                                          )
                                        }
                                        aria-label={`Unban ${targetLabel}`}
                                      >
                                        Unban
                                      </button>
                                    </>
                                  ) : null}
                                </div>
                              ) : null}
                            </div>
                          );
                        })}
                      </div>
                    </div>
                  </li>
                );
              })}
            </ul>
          )}
          {newMessagesWaiting ? (
            <button type="button" className="chat-panel__jump" onClick={scrollTranscriptToBottom}>
              Jump to latest
            </button>
          ) : null}
        </div>
      )}
      {user ? (
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
            placeholder="Share your thoughts"
            value={content}
            onChange={(event) => setContent(event.target.value)}
            disabled={isComposerDisabled}
            rows={3}
            aria-disabled={false}
          />
          <div className="chat-panel__toolbar">
            <button
              type="submit"
              className="primary-button"
              disabled={isComposerDisabled || content.trim().length === 0}
            >
              {sending ? "Sending..." : "Send"}
            </button>
          </div>
        </form>
      ) : (
        <div className="chat-panel__signin-card surface" role="status">
          <p className="muted">Sign in to view and participate in chat.</p>
          <button type="button" className="primary-button" onClick={() => void signIn()}>
            Sign in to chat
          </button>
        </div>
      )}
      {selectedReportMessage && (
        <div
          className="chat-panel__dialog-backdrop"
          role="presentation"
          onClick={closeReportForm}
        >
          <section
            className="chat-panel__dialog surface"
            role="dialog"
            aria-modal="true"
            aria-labelledby="chat-report-title"
            onClick={(event) => event.stopPropagation()}
            ref={reportDialogRef}
          >
            <header className="chat-panel__dialog-header">
              <h4 id="chat-report-title" tabIndex={-1} ref={reportHeadingRef}>Report chat message</h4>
              <button
                type="button"
                className="icon-button"
                onClick={closeReportForm}
                aria-label="Close report dialog"
              >
                x
              </button>
            </header>
            <p className="muted">
              Send this message to moderators for review. Reports are not posted back into chat.
            </p>
            <form
              className="chat-panel__report-form"
              onSubmit={(event) => handleReportSubmit(event, selectedReportMessage)}
              aria-label={`Report chat message ${selectedReportMessage.id}`}
            >
              <label className="chat-panel__report-field" htmlFor={`chat-report-reason-${selectedReportMessage.id}`}>
                <span>Report reason</span>
                <textarea
                  id={`chat-report-reason-${selectedReportMessage.id}`}
                  value={reportReason}
                  onChange={(event) => setReportReason(event.target.value)}
                  placeholder="Reason for report"
                  rows={4}
                />
              </label>
              <div className="chat-panel__dialog-actions">
                <button type="button" className="ghost-button" onClick={closeReportForm} disabled={reportSubmitting}>
                  Cancel
                </button>
                <button type="submit" className="accent-button" disabled={reportSubmitting}>
                  {reportSubmitting ? "Submitting..." : "Submit report"}
                </button>
              </div>
            </form>
          </section>
        </div>
      )}
    </section>
  );
}
