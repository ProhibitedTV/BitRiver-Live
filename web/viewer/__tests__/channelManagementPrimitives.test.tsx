import { fireEvent, screen } from "@testing-library/react";
import type { FormEvent } from "react";
import {
  ChannelSetupCard,
  LivePreviewPanel,
  ObsSettingsPanel,
  ShareLinkPanel,
  buildObsSettingsBlock,
  describeEndpoint,
  emptyScheduleDraft,
  formatCategory,
  fromDateTimeLocalValue,
  getPreferredIngestEndpoint,
  toDateTimeLocalValue,
} from "../components/channel/ChannelManagementPrimitives";
import { renderWithProviders } from "../test/test-utils";
import type { ChannelPlaybackResponse, ManagedChannel } from "../lib/viewer-api";

jest.mock("../components/Player", () => ({
  Player: ({ channelId, live, liveState }: { channelId: string; live?: boolean; liveState?: string }) => (
    <div data-testid="shared-preview-player">
      {JSON.stringify({ channelId, live, liveState })}
    </div>
  ),
}));

const managedChannels: ManagedChannel[] = [
  {
    id: "chan-1",
    ownerId: "creator-1",
    title: "Main Channel",
    category: "Science & Tech",
    tags: ["setup"],
    liveState: "offline",
    createdAt: "2026-03-21T00:00:00.000Z",
    updatedAt: "2026-03-21T00:10:00.000Z",
    streamKey: "sk_live_123",
    ingestEndpoints: ["rtmp://ingest.example.com/live"],
  },
  {
    id: "chan-2",
    ownerId: "creator-1",
    title: "Backup Channel",
    tags: [],
    liveState: "offline",
    createdAt: "2026-03-21T00:00:00.000Z",
    updatedAt: "2026-03-21T00:10:00.000Z",
    streamKey: "sk_live_456",
  },
];

const playback: ChannelPlaybackResponse = {
  channel: {
    id: "chan-1",
    ownerId: "creator-1",
    title: "Main Channel",
    category: "Science & Tech",
    tags: ["setup"],
    liveState: "live",
    currentSessionId: "session-1",
    createdAt: "2026-03-21T00:00:00.000Z",
    updatedAt: "2026-03-21T00:10:00.000Z",
  },
  owner: { id: "creator-1", displayName: "Creator One" },
  profile: {},
  donationAddresses: [],
  live: true,
  follow: { followers: 0, following: false },
  playback: {
    sessionId: "session-1",
    startedAt: "2026-03-21T01:00:00.000Z",
    playbackUrl: "https://cdn.example.com/live.m3u8",
  },
};

describe("channel management primitive helpers", () => {
  it("formats channel metadata and OBS settings consistently", () => {
    expect(formatCategory()).toBe("Uncategorized");
    expect(formatCategory("Science & Tech")).toBe("Science & Tech");
    expect(describeEndpoint("rtmp://ingest.example.com/live", 0)).toBe("Primary ingest");
    expect(describeEndpoint("rtmp://backup.example.com/live", 1)).toBe("Backup ingest");
    expect(describeEndpoint("rtmp://third.example.com/live", 2)).toBe("Ingest 3");
    expect(getPreferredIngestEndpoint(["https://edge.example.com/live", "RTMP://ingest.example.com/live"])).toBe(
      "RTMP://ingest.example.com/live",
    );
    expect(getPreferredIngestEndpoint([])).toBeUndefined();
    expect(buildObsSettingsBlock("rtmp://ingest.example.com/live", "sk_live_123", false)).toBe(
      "Service: Custom\nServer: rtmp://ingest.example.com/live\nStream Key: [hidden - reveal to copy]",
    );
    expect(buildObsSettingsBlock("rtmp://ingest.example.com/live", "sk_live_123", true)).toContain("sk_live_123");
    expect(buildObsSettingsBlock()).toContain("owner access required");
  });

  it("normalizes datetime-local values defensively", () => {
    expect(toDateTimeLocalValue()).toBe("");
    expect(toDateTimeLocalValue("not a date")).toBe("");
    expect(toDateTimeLocalValue("2026-06-05T20:00:00.000Z")).toMatch(/^2026-06-\d{2}T\d{2}:00$/);
    expect(fromDateTimeLocalValue("")).toBeUndefined();
    expect(fromDateTimeLocalValue("not a date")).toBeUndefined();
    expect(fromDateTimeLocalValue("2026-06-05T20:00")).toBe(new Date("2026-06-05T20:00").toISOString());
  });
});

describe("ChannelSetupCard", () => {
  it("renders channel switching, stream title, and schedule controls as reusable primitives", () => {
    const onChannelChange = jest.fn();
    const onTitleDraftChange = jest.fn();
    const onTitleSubmit = jest.fn((event: FormEvent<HTMLFormElement>) => event.preventDefault());
    const onScheduleDraftChange = jest.fn();
    const onScheduleSubmit = jest.fn((event: FormEvent<HTMLFormElement>) => event.preventDefault());
    const onClearSchedule = jest.fn();

    renderWithProviders(
      <ChannelSetupCard
        channelId="chan-1"
        currentChannelTitle="Main Channel"
        currentChannelCategory="Science & Tech"
        managedChannels={managedChannels}
        onChannelChange={onChannelChange}
        titleDraft="New title"
        currentTitle="Main Channel"
        savingTitle={false}
        titleSaved={false}
        onTitleDraftChange={onTitleDraftChange}
        onTitleSubmit={onTitleSubmit}
        scheduleDraft={{
          ...emptyScheduleDraft,
          title: "Friday Night Runs",
          startsAt: "2026-06-05T20:00",
          durationMinutes: "90",
        }}
        savingSchedule={false}
        scheduleSaved={false}
        onScheduleDraftChange={onScheduleDraftChange}
        onScheduleSubmit={onScheduleSubmit}
        onClearSchedule={onClearSchedule}
        managedLoading={false}
      />,
    );

    expect(screen.getByRole("heading", { name: "1) Channel" })).toBeInTheDocument();
    expect(screen.getByLabelText("Current channel")).toHaveValue("Main Channel");
    expect(screen.getByText("Category: Science & Tech")).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Switch channel"), { target: { value: "chan-2" } });
    expect(onChannelChange).toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText("Stream title"), { target: { value: "Updated title" } });
    expect(onTitleDraftChange).toHaveBeenCalledWith("Updated title");
    fireEvent.click(screen.getByRole("button", { name: "Save title" }));
    expect(onTitleSubmit).toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText("Scheduled title"), { target: { value: "Community Showcase" } });
    expect(onScheduleDraftChange).toHaveBeenCalledWith(expect.any(Function));
    fireEvent.submit(screen.getByRole("form", { name: "Update stream schedule" }));
    expect(onScheduleSubmit).toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Clear schedule" }));
    expect(onClearSchedule).toHaveBeenCalled();
  });
});

describe("ObsSettingsPanel", () => {
  it("keeps ingest, stream-key, and OBS copy actions wired through props", () => {
    const onCopyIngestEndpoint = jest.fn();
    const onToggleStreamKey = jest.fn();
    const onCopyKey = jest.fn();
    const onCopyObsSettings = jest.fn();

    renderWithProviders(
      <ObsSettingsPanel
        authLoading={false}
        managedLoading={false}
        isChannelOwner
        streamKeyVisible={false}
        streamKey="sk_live_123"
        preferredIngestEndpoint="rtmp://ingest.example.com/live"
        onCopyIngestEndpoint={onCopyIngestEndpoint}
        onToggleStreamKey={onToggleStreamKey}
        onCopyKey={onCopyKey}
        onCopyObsSettings={onCopyObsSettings}
      />,
    );

    expect(screen.getByRole("heading", { name: "2) Stream settings" })).toBeInTheDocument();
    expect(screen.getByLabelText("Preferred ingest URL")).toHaveValue("rtmp://ingest.example.com/live");
    expect(screen.getByLabelText("Stream key")).toHaveValue("********");

    fireEvent.click(screen.getByRole("button", { name: "Copy URL" }));
    expect(onCopyIngestEndpoint).toHaveBeenCalledWith("rtmp://ingest.example.com/live");
    fireEvent.click(screen.getByRole("button", { name: "Reveal" }));
    expect(onToggleStreamKey).toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Copy key" }));
    expect(onCopyKey).toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Copy OBS settings" }));
    expect(onCopyObsSettings).toHaveBeenCalled();
  });
});

describe("LivePreviewPanel and ShareLinkPanel", () => {
  it("renders the live preview status and delegates refreshes", () => {
    const onRefresh = jest.fn();

    renderWithProviders(
      <LivePreviewPanel
        channelId="chan-1"
        playback={playback}
        testStreamStatus={{
          key: "live",
          label: "Live",
          badgeClassName: "badge badge--live",
          instructions: "Your stream is reaching BitRiver.",
        }}
        testStreamUpdatedAt="2026-03-21T01:05:00.000Z"
        latestSession={{
          id: "session-1",
          channelId: "chan-1",
          startedAt: "2026-03-21T01:00:00.000Z",
          renditions: [],
          peakConcurrent: 0,
        }}
        previewMessage="Preview is live."
        onRefresh={onRefresh}
      />,
    );

    expect(screen.getByRole("heading", { name: "3) Go live" })).toBeInTheDocument();
    expect(screen.getByTestId("test-stream-status-card")).toHaveTextContent("Live");
    expect(screen.getByTestId("shared-preview-player")).toHaveTextContent(
      JSON.stringify({ channelId: "chan-1", live: true, liveState: "live" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Refresh now" }));
    expect(onRefresh).toHaveBeenCalled();
  });

  it("renders the reusable viewer share controls", () => {
    const onCopyViewerLink = jest.fn();

    renderWithProviders(
      <ShareLinkPanel
        viewerPageHref="http://localhost/viewer/channels/chan-1"
        viewerLinkCopyMessage="Copied viewer link"
        onCopyViewerLink={onCopyViewerLink}
      />,
    );

    expect(screen.getByRole("heading", { name: "4) Share" })).toBeInTheDocument();
    expect(screen.getByLabelText("Viewer link")).toHaveValue("http://localhost/viewer/channels/chan-1");
    fireEvent.click(screen.getByRole("button", { name: "Copy viewer link" }));
    expect(onCopyViewerLink).toHaveBeenCalled();
    expect(screen.getByRole("link", { name: "Open viewer" })).toHaveAttribute(
      "href",
      "http://localhost/viewer/channels/chan-1",
    );
    expect(screen.getByText("Copied viewer link")).toBeInTheDocument();
  });
});
