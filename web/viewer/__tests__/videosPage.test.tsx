import { render, screen, waitFor } from "@testing-library/react";
import { viewerApiMocks } from "../test/test-utils";
import VideosPage from "../app/videos/page";

const fetchRecommendedChannelsMock = viewerApiMocks.fetchRecommendedChannels;
const fetchTrendingChannelsMock = viewerApiMocks.fetchTrendingChannels;
const fetchLiveNowChannelsMock = viewerApiMocks.fetchLiveNowChannels;
const fetchChannelVodsMock = viewerApiMocks.fetchChannelVods;

const channel = {
  channel: {
    id: "chan-1",
    ownerId: "owner-1",
    title: "Deep Space Beats",
    category: "Music",
    tags: ["lofi"],
    liveState: "offline",
    createdAt: new Date("2023-10-20T10:00:00Z").toISOString(),
    updatedAt: new Date("2023-10-21T11:00:00Z").toISOString(),
  },
  owner: { id: "owner-1", displayName: "DJ Nova" },
  profile: {},
  live: false,
  followerCount: 12,
};

describe("VideosPage", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    const directoryResponse = {
      channels: [channel],
      generatedAt: new Date("2023-10-21T11:00:00Z").toISOString(),
    } as any;
    fetchRecommendedChannelsMock.mockResolvedValue(directoryResponse);
    fetchTrendingChannelsMock.mockResolvedValue(directoryResponse);
    fetchLiveNowChannelsMock.mockResolvedValue(directoryResponse);
  });

  test("renders replay shelves from existing public channel VODs", async () => {
    fetchChannelVodsMock.mockResolvedValue({
      channelId: "chan-1",
      items: [
        {
          id: "vod-1",
          title: "Late Night Replay",
          durationSeconds: 3660,
          publishedAt: new Date("2023-10-22T02:00:00Z").toISOString(),
        },
      ],
    } as any);

    render(<VideosPage />);

    await waitFor(() => expect(fetchChannelVodsMock).toHaveBeenCalledWith("chan-1"));

    expect(await screen.findByRole("heading", { level: 2, name: "Recent replays" })).toBeInTheDocument();
    expect(screen.getByText("Late Night Replay")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /open replays/i })).toHaveAttribute("href", "/channels/chan-1?tab=videos");
    expect(screen.getByRole("link", { name: /view channel videos/i })).toHaveAttribute(
      "href",
      "/channels/chan-1?tab=videos",
    );
  });

  test("shows a recovery state when no public replays are available", async () => {
    fetchChannelVodsMock.mockResolvedValue({
      channelId: "chan-1",
      items: [],
    } as any);

    render(<VideosPage />);

    await waitFor(() => expect(fetchChannelVodsMock).toHaveBeenCalledWith("chan-1"));

    expect(await screen.findByText(/no public replays yet/i)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /watch live channels/i })).toHaveAttribute("href", "/browse");
    expect(screen.getByRole("link", { name: /open creator setup/i })).toHaveAttribute(
      "href",
      "/creator/getting-started",
    );
  });
});
