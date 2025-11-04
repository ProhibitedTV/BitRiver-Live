import userEvent from "@testing-library/user-event";
import { render, screen, waitFor, within } from "@testing-library/react";
import DirectoryPage from "../app/page";
import { fetchDirectory, searchDirectory } from "../lib/viewer-api";

jest.mock("../lib/viewer-api", () => ({
  ...jest.requireActual("../lib/viewer-api"),
  fetchDirectory: jest.fn(),
  searchDirectory: jest.fn()
}));

const fetchDirectoryMock = fetchDirectory as jest.MockedFunction<typeof fetchDirectory>;
const searchDirectoryMock = searchDirectory as jest.MockedFunction<typeof searchDirectory>;

const baseDirectoryResponse = {
  channels: [
    {
      channel: {
        id: "chan-1",
        ownerId: "owner-1",
        title: "Deep Space Beats",
        category: "Music",
        tags: ["lofi", "ambient"],
        liveState: "live",
        currentSessionId: "session-1",
        createdAt: new Date("2023-10-20T10:00:00Z").toISOString(),
        updatedAt: new Date("2023-10-21T11:00:00Z").toISOString()
      },
      owner: {
        id: "owner-1",
        displayName: "DJ Nova"
      },
      profile: {
        bio: "Streaming vinyl sets from a solar-powered cabin.",
        avatarUrl: undefined,
        bannerUrl: undefined
      },
      live: true,
      followerCount: 12
    }
  ],
  generatedAt: new Date("2023-10-21T11:00:00Z").toISOString()
};

const searchDirectoryResponse = {
  channels: [
    {
      channel: {
        id: "chan-2",
        ownerId: "owner-2",
        title: "Retro Speedruns",
        category: "Gaming",
        tags: ["speedrun", "retro"],
        liveState: "live",
        currentSessionId: "session-2",
        createdAt: new Date("2023-10-18T18:00:00Z").toISOString(),
        updatedAt: new Date("2023-10-21T12:30:00Z").toISOString()
      },
      owner: {
        id: "owner-2",
        displayName: "PixelPro"
      },
      profile: {
        bio: "Tool-assisted runs from the golden age of consoles.",
        avatarUrl: undefined,
        bannerUrl: undefined
      },
      live: true,
      followerCount: 8
    }
  ],
  generatedAt: new Date("2023-10-21T12:30:00Z").toISOString()
};

describe("DirectoryPage", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  test("loads directory entries and renders channel cards", async () => {
    fetchDirectoryMock.mockResolvedValueOnce(baseDirectoryResponse as any);

    render(<DirectoryPage />);

    await waitFor(() => expect(fetchDirectoryMock).toHaveBeenCalledTimes(1));

    const heading = await screen.findByRole("heading", { level: 3, name: "Deep Space Beats" });
    const card = heading.closest("article");
    expect(card).toBeTruthy();
    const withinCard = within(card!);
    expect(withinCard.getByText(/dj nova/i)).toBeInTheDocument();
    expect(withinCard.getByText(/followers:\s*12/i)).toBeInTheDocument();
    expect(withinCard.getByText(/12 followers/i)).toBeInTheDocument();
    expect(withinCard.queryByText(/12 viewers/i)).not.toBeInTheDocument();
  });

  test("performs a search and swaps the directory results", async () => {
    fetchDirectoryMock.mockResolvedValueOnce(baseDirectoryResponse as any);
    searchDirectoryMock.mockResolvedValueOnce(searchDirectoryResponse as any);
    const user = userEvent.setup();

    render(<DirectoryPage />);

    await screen.findByRole("heading", { level: 3, name: "Deep Space Beats" });

    await user.clear(screen.getByRole("searchbox", { name: /search channels/i }));
    await user.type(screen.getByRole("searchbox", { name: /search channels/i }), "retro");
    await user.click(screen.getByRole("button", { name: /search/i }));

    await waitFor(() => {
      expect(searchDirectoryMock).toHaveBeenCalledWith("retro");
    });

    expect(await screen.findByRole("heading", { level: 3, name: "Retro Speedruns" })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { level: 3, name: "Deep Space Beats" })).not.toBeInTheDocument();
  });

  test("surfaces a friendly error when the directory fails to load", async () => {
    fetchDirectoryMock.mockRejectedValueOnce(new Error("Gateway timeout"));

    render(<DirectoryPage />);

    await waitFor(() => expect(fetchDirectoryMock).toHaveBeenCalled());
    expect(await screen.findByText(/unable to load directory|gateway timeout/i)).toBeInTheDocument();
  });
});
