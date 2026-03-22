import userEvent from "@testing-library/user-event";
import { screen, waitFor } from "@testing-library/react";
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
    updateChannelMock.mockResolvedValue({
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
    } as any);
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: {
        writeText: jest.fn().mockResolvedValue(undefined),
      },
    });
  });

  it("renders the guided flow in order and exposes the key live-setup affordances", async () => {
    const user = userEvent.setup();
    const { reload } = renderCreatorLivePage();

    await waitFor(() => {
      expect(fetchManagedChannelsMock).toHaveBeenCalled();
      expect(fetchChannelSessionsMock).toHaveBeenCalled();
    });

    expect(screen.getAllByRole("heading", { level: 3 }).map((heading) => heading.textContent)).toEqual([
      "1) Channel",
      "2) OBS Setup",
      "3) Test Stream",
      "4) Preview",
      "5) Share",
    ]);

    expect(screen.getByLabelText("Current channel")).toHaveValue("Main Channel");
    expect(screen.getByLabelText("Switch channel")).toHaveValue("chan-1");
    expect(screen.getByLabelText("Preferred ingest URL")).toHaveValue("rtmp://ingest.example.com/live");
    expect(screen.getByLabelText("Stream key")).toHaveValue("••••••••");
    expect(screen.getByLabelText("OBS settings block")).toHaveValue(
      "Service: Custom\nServer: rtmp://ingest.example.com/live\nStream Key: [hidden - reveal to copy]",
    );
    expect(screen.getByText(/this page refreshes the current live signals every 4 seconds/i)).toBeInTheDocument();
    expect(screen.getByText(/BitRiver is receiving your stream, but the preview player is still warming up/i)).toBeInTheDocument();
    expect(screen.getByTestId("creator-preview-player")).toHaveTextContent(
      JSON.stringify({ channelId: "chan-1", live: false, liveState: "starting" }),
    );
    expect(screen.getByLabelText("Viewer link")).toHaveValue("http://localhost/channels/chan-1");
    expect(screen.getByRole("button", { name: "Copy key" })).toBeEnabled();
    expect(screen.getByTestId("copy-preferred-ingest-endpoint")).toBeEnabled();
    expect(screen.getByRole("button", { name: "Copy OBS settings" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Copy viewer link" })).toBeEnabled();

    await user.click(screen.getByRole("button", { name: "Reveal" }));
    await waitFor(() => expect(screen.getByLabelText("Stream key")).toHaveValue("sk_live_123"));
    expect(screen.getByLabelText("OBS settings block")).toHaveValue(
      "Service: Custom\nServer: rtmp://ingest.example.com/live\nStream Key: sk_live_123",
    );

    await user.click(screen.getByRole("button", { name: "Refresh now" }));
    await waitFor(() => expect(reload).toHaveBeenCalledWith(true));

    expect(screen.getByRole("link", { name: "Open viewer" })).toHaveAttribute("href", "http://localhost/channels/chan-1");
  });
});
