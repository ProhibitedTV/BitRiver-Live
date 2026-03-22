import type { StreamSession } from "../../../../lib/viewer-api";

export type CreatorGoLiveStatus = {
  key: "waiting" | "live" | "reconnecting" | "offline-unknown";
  label: "Waiting for stream" | "Live" | "Reconnecting" | "Offline / Unknown";
  badgeClassName: string;
  instructions: string;
  lastTransitionAt?: string;
  reason?: string;
};

export const CREATOR_STATUS_LABELS = {
  waiting: "Waiting for stream",
  live: "Live",
  reconnecting: "Reconnecting",
  offlineUnknown: "Offline / Unknown",
} as const;

function sessionIsActive(session: StreamSession | undefined): boolean {
  return Boolean(session && !session.endedAt);
}

export function deriveCreatorGoLiveStatus(
  live: boolean,
  liveState: string | undefined,
  currentSessionId: string | undefined,
  latestSession: StreamSession | undefined,
): CreatorGoLiveStatus {
  if (live || liveState === "live") {
    return {
      key: "live",
      label: CREATOR_STATUS_LABELS.live,
      badgeClassName: "badge badge--live",
      instructions: "Your stream is reaching BitRiver. Keep OBS running while you confirm the preview below.",
      lastTransitionAt: latestSession?.startedAt,
    };
  }

  if (
    liveState === "starting" ||
    (liveState === "offline" && currentSessionId && sessionIsActive(latestSession))
  ) {
    return {
      key: "reconnecting",
      label: CREATOR_STATUS_LABELS.reconnecting,
      badgeClassName: "badge badge--ingesting",
      instructions: "We can see your encoder. Keep streaming while BitRiver finishes reconnecting the preview.",
      lastTransitionAt: latestSession?.startedAt,
      reason:
        liveState === "starting"
          ? "Encoder connected; playback is still warming up."
          : "Recent ingest activity is still settling.",
    };
  }

  if (liveState === "offline") {
    return {
      key: "waiting",
      label: CREATOR_STATUS_LABELS.waiting,
      badgeClassName: "badge badge--muted",
      instructions: "Open OBS, paste the server URL and stream key, then click Start Streaming to begin your test.",
      lastTransitionAt: latestSession?.endedAt ?? latestSession?.startedAt,
    };
  }

  return {
    key: "offline-unknown",
    label: CREATOR_STATUS_LABELS.offlineUnknown,
    badgeClassName: "badge badge--error",
    instructions: "We could not confirm your stream status yet. Refresh once more and double-check the OBS details on this page.",
    reason: liveState
      ? `Unexpected server live_state: ${liveState}`
      : "Server did not provide live_state",
  };
}
