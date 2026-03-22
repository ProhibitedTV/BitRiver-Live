import { deriveCreatorGoLiveStatus } from "../app/creator/live/[channelId]/stream-status";

describe("deriveCreatorGoLiveStatus", () => {
  it("maps starting to reconnecting", () => {
    const status = deriveCreatorGoLiveStatus(false, "starting", "session-1", {
      id: "session-1",
      channelId: "channel-1",
      startedAt: "2026-01-01T00:00:00.000Z",
      renditions: [],
      peakConcurrent: 0,
    });

    expect(status).toEqual({
      key: "reconnecting",
      label: "Reconnecting",
      badgeClassName: "badge badge--ingesting",
      instructions: "We can see your encoder. Keep streaming while BitRiver finishes reconnecting the preview.",
      lastTransitionAt: "2026-01-01T00:00:00.000Z",
      reason: "Encoder connected; playback is still warming up.",
    });
  });

  it("maps live to live badge", () => {
    const status = deriveCreatorGoLiveStatus(true, "live", "session-1", {
      id: "session-1",
      channelId: "channel-1",
      startedAt: "2026-01-01T00:00:00.000Z",
      renditions: [],
      peakConcurrent: 0,
    });

    expect(status).toEqual({
      key: "live",
      label: "Live",
      badgeClassName: "badge badge--live",
      instructions: "Your stream is reaching BitRiver. Keep OBS running while you confirm the preview below.",
      lastTransitionAt: "2026-01-01T00:00:00.000Z",
    });
  });

  it("maps offline with no active ingest to waiting", () => {
    const status = deriveCreatorGoLiveStatus(false, "offline", undefined, {
      id: "session-1",
      channelId: "channel-1",
      startedAt: "2026-01-01T00:00:00.000Z",
      endedAt: "2026-01-01T01:00:00.000Z",
      renditions: [],
      peakConcurrent: 0,
    });

    expect(status).toEqual({
      key: "waiting",
      label: "Waiting for stream",
      badgeClassName: "badge badge--muted",
      instructions: "Open OBS, paste the server URL and stream key, then click Start Streaming to begin your test.",
      lastTransitionAt: "2026-01-01T01:00:00.000Z",
    });
  });

  it("treats active offline ingest as reconnecting", () => {
    const status = deriveCreatorGoLiveStatus(false, "offline", "session-1", {
      id: "session-1",
      channelId: "channel-1",
      startedAt: "2026-01-01T00:00:00.000Z",
      renditions: [],
      peakConcurrent: 0,
    });

    expect(status).toEqual({
      key: "reconnecting",
      label: "Reconnecting",
      badgeClassName: "badge badge--ingesting",
      instructions: "We can see your encoder. Keep streaming while BitRiver finishes reconnecting the preview.",
      lastTransitionAt: "2026-01-01T00:00:00.000Z",
      reason: "Recent ingest activity is still settling.",
    });
  });

  it("maps unknown value to offline or unknown", () => {
    const status = deriveCreatorGoLiveStatus(false, "paused", undefined, undefined);

    expect(status).toEqual({
      key: "offline-unknown",
      label: "Offline / Unknown",
      badgeClassName: "badge badge--error",
      instructions: "We could not confirm your stream status yet. Refresh once more and double-check the OBS details on this page.",
      reason: "Unexpected server live_state: paused",
    });
  });
});
