import type { StreamSession } from "../../../../lib/viewer-api";

export type ControlCentreStreamStatus = {
  label: "Offline" | "Reconnecting" | "Live" | "Ended" | "Error";
  badgeClassName: string;
  lastTransitionAt?: string;
  reason?: string;
};

export const CREATOR_STATUS_LABELS = {
  offline: "Offline",
  starting: "Reconnecting",
  live: "Live",
  ended: "Ended",
  error: "Error",
} as const;

export function deriveControlCentreStatus(
  liveState: string | undefined,
  currentSessionId: string | undefined,
  latestSession: StreamSession | undefined,
): ControlCentreStreamStatus {
  if (liveState === "starting") {
    return {
      label: CREATOR_STATUS_LABELS.starting,
      badgeClassName: "badge badge--ingesting",
      lastTransitionAt: latestSession?.startedAt,
      reason: "Encoder connected; stream is still provisioning.",
    };
  }

  if (liveState === "live") {
    return {
      label: CREATOR_STATUS_LABELS.live,
      badgeClassName: "badge badge--live",
      lastTransitionAt: latestSession?.startedAt,
    };
  }

  if (liveState === "offline") {
    if (latestSession?.endedAt) {
      return {
        label: CREATOR_STATUS_LABELS.ended,
        badgeClassName: "badge badge--ended",
        lastTransitionAt: latestSession.endedAt,
        reason: "Ended normally.",
      };
    }

    if (currentSessionId && !latestSession) {
      return {
        label: CREATOR_STATUS_LABELS.error,
        badgeClassName: "badge badge--error",
        reason: "Ingest lost before session details were persisted.",
      };
    }

    if (currentSessionId && latestSession && latestSession.id !== currentSessionId) {
      return {
        label: CREATOR_STATUS_LABELS.error,
        badgeClassName: "badge badge--error",
        lastTransitionAt: latestSession.startedAt,
        reason: "Ingest lost: channel session signal is out of sync.",
      };
    }

    return {
      label: CREATOR_STATUS_LABELS.offline,
      badgeClassName: "badge badge--muted",
      lastTransitionAt: latestSession?.endedAt,
    };
  }

  if (liveState === "ended") {
    return {
      label: CREATOR_STATUS_LABELS.ended,
      badgeClassName: "badge badge--ended",
      lastTransitionAt: latestSession?.endedAt ?? latestSession?.startedAt,
      reason: "Stream ended and is awaiting the next ingest session.",
    };
  }

  return {
    label: CREATOR_STATUS_LABELS.error,
    badgeClassName: "badge badge--error",
    reason: liveState
      ? `Unexpected server live_state: ${liveState}`
      : "Server did not provide live_state",
  };
}
