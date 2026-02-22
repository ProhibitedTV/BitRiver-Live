import { deriveControlCentreStatus } from "../app/creator/live/[channelId]/page";

describe("deriveControlCentreStatus", () => {
  it("maps starting to ingesting", () => {
    const status = deriveControlCentreStatus("starting", "session-1", {
      id: "session-1",
      channelId: "channel-1",
      startedAt: "2026-01-01T00:00:00.000Z",
      renditions: [],
      peakConcurrent: 0,
    });

    expect(status).toEqual({
      label: "Ingesting",
      badgeClassName: "badge badge--ingesting",
      lastTransitionAt: "2026-01-01T00:00:00.000Z",
      reason: "Encoder connected; stream is still provisioning.",
    });
  });

  it("maps live to live badge", () => {
    const status = deriveControlCentreStatus("live", "session-1", {
      id: "session-1",
      channelId: "channel-1",
      startedAt: "2026-01-01T00:00:00.000Z",
      renditions: [],
      peakConcurrent: 0,
    });

    expect(status).toEqual({
      label: "Live",
      badgeClassName: "badge badge--live",
      lastTransitionAt: "2026-01-01T00:00:00.000Z",
    });
  });

  it("maps offline with ended session to ended", () => {
    const status = deriveControlCentreStatus("offline", "session-1", {
      id: "session-1",
      channelId: "channel-1",
      startedAt: "2026-01-01T00:00:00.000Z",
      endedAt: "2026-01-01T01:00:00.000Z",
      renditions: [],
      peakConcurrent: 0,
    });

    expect(status).toEqual({
      label: "Ended",
      badgeClassName: "badge badge--ended",
      lastTransitionAt: "2026-01-01T01:00:00.000Z",
      reason: "Ended normally.",
    });
  });

  it("maps ended to ended", () => {
    const status = deriveControlCentreStatus("ended", "session-1", {
      id: "session-1",
      channelId: "channel-1",
      startedAt: "2026-01-01T00:00:00.000Z",
      endedAt: "2026-01-01T01:00:00.000Z",
      renditions: [],
      peakConcurrent: 0,
    });

    expect(status).toEqual({
      label: "Ended",
      badgeClassName: "badge badge--ended",
      lastTransitionAt: "2026-01-01T01:00:00.000Z",
      reason: "Stream ended and is awaiting the next ingest session.",
    });
  });

  it("maps unknown value to unexpected server state", () => {
    const status = deriveControlCentreStatus("paused", undefined, undefined);

    expect(status).toEqual({
      label: "Error",
      badgeClassName: "badge badge--error",
      reason: "Unexpected server live_state: paused",
    });
  });
});
