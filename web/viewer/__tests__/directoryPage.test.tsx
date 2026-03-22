import { mockRouter, resetRouterMocks, viewerApiMocks } from "../test/test-utils";
import userEvent from "@testing-library/user-event";
import { render, screen, waitFor, within } from "@testing-library/react";
import DirectoryPage from "../app/page";
import { DirectoryDataBoundary, DirectoryPageShell } from "../app/directory-page-shell";
import { directoryInputMatrix } from "../test/directory-input-matrix";
import { normalizeDirectoryQuery } from "../lib/directory-state";

const fetchDirectoryMock = viewerApiMocks.fetchDirectory;
const searchDirectoryMock = viewerApiMocks.searchDirectory;
const fetchFeaturedChannelsMock = viewerApiMocks.fetchFeaturedChannels;
const fetchFollowingChannelsMock = viewerApiMocks.fetchFollowingChannels;
const fetchLiveNowChannelsMock = viewerApiMocks.fetchLiveNowChannels;

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
  const renderResolvedDirectoryPage = async (query?: string) => {
    const normalizedQuery = normalizeDirectoryQuery(query ?? "");
    const boundary = await DirectoryDataBoundary({ query: normalizedQuery });
    return render(<DirectoryPageShell query={normalizedQuery}>{boundary}</DirectoryPageShell>);
  };

  beforeEach(() => {
    jest.clearAllMocks();
    resetRouterMocks();
    const sliceResponse = {
      channels: [],
      generatedAt: new Date("2023-10-21T11:00:00Z").toISOString(),
    } as any;
    fetchFeaturedChannelsMock.mockResolvedValue(sliceResponse);
    fetchFollowingChannelsMock.mockResolvedValue(sliceResponse);
    fetchLiveNowChannelsMock.mockResolvedValue(sliceResponse);
  });

  test("loads directory entries and renders channel cards", async () => {
    fetchDirectoryMock.mockResolvedValueOnce(baseDirectoryResponse as any);

    await renderResolvedDirectoryPage();

    await waitFor(() => expect(fetchDirectoryMock).toHaveBeenCalledTimes(1));
    const quickJumpNav = screen.getByRole("navigation", { name: /quick jump links/i });
    expect(within(quickJumpNav).getByRole("link", { name: /top categories/i })).toHaveAttribute("href", "#top-categories");
    expect(within(quickJumpNav).getByRole("link", { name: /trending now/i })).toHaveAttribute("href", "#trending-now");
    expect(within(quickJumpNav).getByRole("link", { name: /live now/i })).toHaveAttribute("href", "#live-now");

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

    const { rerender } = await renderResolvedDirectoryPage();

    await screen.findByRole("heading", { level: 3, name: "Deep Space Beats" });

    await user.clear(screen.getByRole("searchbox", { name: /search channels/i }));
    await user.type(screen.getByRole("searchbox", { name: /search channels/i }), "retro");
    await user.click(screen.getByRole("button", { name: /search/i }));

    const searchBoundary = await DirectoryDataBoundary({ query: "retro" });
    rerender(<DirectoryPageShell query="retro">{searchBoundary}</DirectoryPageShell>);

    await waitFor(() => {
      expect(searchDirectoryMock).toHaveBeenCalledWith("retro");
    });

    expect(await screen.findByText("Retro Speedruns")).toBeInTheDocument();
    expect(screen.queryByRole("heading", { level: 3, name: "Deep Space Beats" })).not.toBeInTheDocument();
  });

  test("clearing search returns to the default directory route", async () => {
    fetchDirectoryMock.mockResolvedValueOnce(baseDirectoryResponse as any);
    const user = userEvent.setup();

    await renderResolvedDirectoryPage("retro");

    const clearButton = await screen.findByRole("button", { name: /clear/i });
    await user.click(clearButton);

    expect(mockRouter.replace).toHaveBeenCalledWith("/");
  });


  test.each(directoryInputMatrix)("applies shared directory loading behavior for $label", async ({ query, normalized, mode, errorMessage }) => {
    if (mode === "search") {
      searchDirectoryMock.mockResolvedValueOnce(searchDirectoryResponse as any);
    } else if (mode === "error") {
      searchDirectoryMock.mockRejectedValueOnce(new Error(errorMessage));
    } else {
      fetchDirectoryMock.mockResolvedValueOnce(baseDirectoryResponse as any);
    }

    await renderResolvedDirectoryPage(query);

    if (mode === "search") {
      await waitFor(() => expect(searchDirectoryMock).toHaveBeenCalledWith(normalized));
      expect(await screen.findByText("Retro Speedruns")).toBeInTheDocument();
      expect(fetchDirectoryMock).not.toHaveBeenCalled();
      return;
    }

    if (mode === "error") {
      await waitFor(() => expect(searchDirectoryMock).toHaveBeenCalledWith(normalized));
      expect(await screen.findByRole("alert")).toHaveTextContent(errorMessage);
      return;
    }

    await waitFor(() => expect(fetchDirectoryMock).toHaveBeenCalledTimes(1));
    expect(await screen.findByText("Deep Space Beats")).toBeInTheDocument();
    expect(searchDirectoryMock).not.toHaveBeenCalled();
  });

  test("gracefully handles directory loading errors", async () => {
    fetchDirectoryMock.mockRejectedValueOnce(new Error("Gateway timeout"));

    await renderResolvedDirectoryPage();

    await waitFor(() => expect(fetchDirectoryMock).toHaveBeenCalled());
    expect(screen.getByText(/browse the directory/i)).toBeInTheDocument();
  });

  test("shows an empty following message for authenticated users with no follows", async () => {
    fetchDirectoryMock.mockResolvedValueOnce(baseDirectoryResponse as any);

    await renderResolvedDirectoryPage();

    expect(await screen.findByText(/not following any channels yet/i)).toBeInTheDocument();
    expect(screen.queryByText(/sign in to see channels you follow/i)).not.toBeInTheDocument();
  });

  test("prompts guests to sign in when following data is unauthenticated", async () => {
    fetchDirectoryMock.mockResolvedValueOnce(baseDirectoryResponse as any);
    fetchFollowingChannelsMock.mockRejectedValueOnce(new Error("unauthorized"));

    await renderResolvedDirectoryPage();

    expect(await screen.findByText(/sign in to see channels you follow/i)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /sign in/i })).toHaveAttribute("href", "/login");
    expect(screen.queryByText(/not following any channels yet/i)).not.toBeInTheDocument();
  });

  test("keeps DirectoryPage lightweight and normalizes query before passing to shell", async () => {
    const page = DirectoryPage({ searchParams: { q: "   retro   " } });

    expect(page.type).toBe(DirectoryPageShell);
    expect(page.props.query).toBe("retro");
  });

  test("shows suspense fallback content while boundary content is pending", async () => {
    const never = new Promise<never>(() => undefined);

    function PendingBoundary() {
      throw never;
    }

    render(
      <DirectoryPageShell query="retro">
        <PendingBoundary />
      </DirectoryPageShell>
    );

    expect(screen.getAllByText(/loading channels/i).length).toBeGreaterThan(0);
  });
});
