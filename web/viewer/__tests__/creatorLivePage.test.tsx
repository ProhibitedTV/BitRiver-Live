import userEvent from "@testing-library/user-event";
import { fireEvent, screen, waitFor } from "@testing-library/react";
import { mockAuthenticatedUser, renderWithProviders, viewerApiMocks } from "../test/test-utils";
import CreatorLivePage from "../app/creator/live/[channelId]/page";
import { CreatorChannelProvider } from "../hooks/useCreatorChannel";

jest.mock("../hooks/useAuth");
jest.mock("../components/Player", () => ({
  Player: ({ channelId, live, liveState }: { channelId: string; live?: boolean; liveState?: string }) => (
    <div data-testid="creator-preview-player">
      {JSON.stringify({ channelId, live, liveState })}
    </div>
  ),
}));

const fetchManagedChannelsMock = viewerApiMocks.fetchManagedChannels;
const fetchChannelSessionsMock = viewerApiMocks.fetchChannelSessions;
const updateChannelMock = viewerApiMocks.updateChannel;
const originalViewerBasePath = process.env.NEXT_PUBLIC_VIEWER_BASE_PATH;

function renderCreatorLivePage() {
  const reload = jest.fn().mockResolvedValue(undefined);

  renderWithProviders(
    <CreatorChannelProvider
      value={{
        channelId: "chan-1",
        loading: false,
        error: undefined,
        reload,
        playback: {
          channel: {
            id: "chan-1",
            ownerId: "creator-1",
            title: "Main Channel",
            category: "Science & Tech",
            tags: ["setup"],
            liveState: "starting",
            currentSessionId: "session-1",
            createdAt: "2026-03-21T00:00:00.000Z",
            updatedAt: "2026-03-21T00:10:00.000Z",
          },
          owner: { id: "creator-1", displayName: "Creator One" },
          profile: { bio: "Bio" },
          donationAddresses: [],
          live: false,
          follow: { followers: 0, following: false },
        },
      }}
    >
      <CreatorLivePage />
    </CreatorChannelProvider>,
  );

  return { reload };
}

describe("CreatorLivePage", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    process.env.NEXT_PUBLIC_VIEWER_BASE_PATH = "/viewer";
    mockAuthenticatedUser({ id: "creator-1" });
    fetchManagedChannelsMock.mockResolvedValue([
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
        ingestEndpoints: ["rtmp://ingest.example.com/live", "rtmp://backup.example.com/live"],
      },
      {
        id: "chan-2",
        ownerId: "creator-1",
        title: "Backup Channel",
        category: "Gaming",
        tags: [],
        liveState: "offline",
        createdAt: "2026-03-21T00:00:00.000Z",
        updatedAt: "2026-03-21T00:10:00.000Z",
        streamKey: "sk_live_456",
        ingestEndpoints: ["rtmp://backup-channel.example.com/live"],
      },
    ] as any);
    fetchChannelSessionsMock.mockResolvedValue([
      {
        id: "session-1",
        channelId: "chan-1",
        startedAt: "2026-03-21T01:00:00.000Z",
        renditions: [],
        peakConcurrent: 0,
      },
    ] as any);
    updateChannelMock.mockImplementation(async (_channelId: string, payload: any) => ({
      id: "chan-1",
      ownerId: "creator-1",
      title: payload.title ?? "Main Channel",
      category: "Science & Tech",
      tags: ["setup"],
      schedule:
        payload.schedule?.map((entry: any, index: number) => ({
          id: entry.id ?? `schedule-${index + 1}`,
          title: entry.title,
          startsAt: entry.startsAt,
          durationMinutes: entry.durationMinutes,
          description: entry.description,
          createdAt: "2026-03-21T00:00:00.000Z",
          updatedAt: "2026-03-21T00:10:00.000Z",
        })) ?? [],
      liveState: "offline",
      createdAt: "2026-03-21T00:00:00.000Z",
      updatedAt: "2026-03-21T00:10:00.000Z",
      streamKey: "sk_live_123",
      ingestEndpoints: ["rtmp://ingest.example.com/live", "rtmp://backup.example.com/live"],
    }));
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: {
        writeText: jest.fn().mockResolvedValue(undefined),
      },
    });
  });

  afterAll(() => {
    if (originalViewerBasePath === undefined) {
      delete process.env.NEXT_PUBLIC_VIEWER_BASE_PATH;
      return;
    }
    process.env.NEXT_PUBLIC_VIEWER_BASE_PATH = originalViewerBasePath;
  });

  it("renders the guided flow in order and exposes the key live-setup affordances", async () => {
    const user = userEvent.setup();
    const { reload } = renderCreatorLivePage();

    await waitFor(() => {
      expect(fetchManagedChannelsMock).toHaveBeenCalled();
      expect(fetchChannelSessionsMock).toHaveBeenCalled();
    });

    expect(screen.getByRole("heading", { level: 2, name: "Main Channel studio" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Public preview" })).toHaveAttribute("href", "/channels/chan-1");
    expect(screen.getByRole("link", { name: "Go live" })).toHaveAttribute("href", "/creator/live/chan-1");
    expect(screen.getByRole("link", { name: "Uploads" })).toHaveAttribute("href", "/creator/uploads/chan-1");
    expect(screen.getByRole("link", { name: "Schedule" })).toHaveAttribute("href", "/creator/live/chan-1#channel-schedule");
    expect(screen.getByRole("link", { name: "Share link" })).toHaveAttribute("href", "/creator/live/chan-1#channel-share");
    expect(screen.getAllByRole("heading", { level: 3 }).map((heading) => heading.textContent)).toEqual([
      "1) Channel",
      "2) Stream settings",
      "3) Go live",
      "4) Share",
    ]);

    expect(screen.getByLabelText("Current channel")).toHaveValue("Main Channel");
    expect(screen.getByLabelText("Scheduled title")).toHaveValue("Main Channel");
    expect(screen.getByLabelText("Switch channel")).toHaveValue("chan-1");
    expect(screen.getByLabelText("Preferred ingest URL")).toHaveValue("rtmp://ingest.example.com/live");
    expect(screen.getByLabelText("Stream key")).toHaveValue("********");
    expect(screen.getByText(/service: custom/i)).toBeInTheDocument();
    expect(screen.getByText(/server: rtmp:\/\/ingest\.example\.com\/live/i)).toBeInTheDocument();
    expect(screen.getByText(/stream key: reveal or copy above/i)).toBeInTheDocument();
    expect(screen.getByText(/start obs; preview appears when playback is ready/i)).toBeInTheDocument();
    expect(screen.getByText(/receiving stream\. keep obs running/i)).toBeInTheDocument();
    expect(screen.getByTestId("creator-preview-player")).toHaveTextContent(
      JSON.stringify({ channelId: "chan-1", live: false, liveState: "starting" }),
    );
    expect(screen.getByLabelText("Viewer link")).toHaveValue("http://localhost/viewer/channels/chan-1");
    expect(screen.getByRole("button", { name: "Copy key" })).toBeEnabled();
    expect(screen.getByTestId("copy-preferred-ingest-endpoint")).toBeEnabled();
    expect(screen.getByRole("button", { name: "Copy OBS settings" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Copy viewer link" })).toBeEnabled();

    await user.click(screen.getByRole("button", { name: "Reveal" }));
    await waitFor(() => expect(screen.getByLabelText("Stream key")).toHaveValue("sk_live_123"));

    await user.click(screen.getByRole("button", { name: "Refresh now" }));
    await waitFor(() => expect(reload).toHaveBeenCalledWith(true));

    fireEvent.change(screen.getByLabelText("Scheduled title"), { target: { value: "Friday Night Runs" } });
    fireEvent.change(screen.getByLabelText("Start time"), { target: { value: "2026-06-05T20:00" } });
    fireEvent.change(screen.getByLabelText("Duration minutes"), { target: { value: "90" } });
    fireEvent.change(screen.getByLabelText("Description"), { target: { value: "Community speedrun showcase" } });
    fireEvent.submit(screen.getByRole("form", { name: "Update stream schedule" }));

    await waitFor(() =>
      expect(updateChannelMock).toHaveBeenCalledWith(
        "chan-1",
        expect.objectContaining({
          schedule: [
            expect.objectContaining({
              title: "Friday Night Runs",
              durationMinutes: 90,
              description: "Community speedrun showcase",
            }),
          ],
        }),
      ),
    );
    expect(await screen.findByText("Schedule updated.")).toBeInTheDocument();

    expect(screen.getByRole("link", { name: "Open viewer" })).toHaveAttribute(
      "href",
      "http://localhost/viewer/channels/chan-1",
    );
  });
});
