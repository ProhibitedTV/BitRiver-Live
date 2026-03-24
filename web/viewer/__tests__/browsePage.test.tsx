import { render, screen, waitFor } from "@testing-library/react";
import { resetRouterMocks, setMockPathname, viewerApiMocks } from "../test/test-utils";
import BrowsePage from "../app/browse/page";
import { directoryInputMatrix } from "../test/directory-input-matrix";

const fetchDirectoryMock = viewerApiMocks.fetchDirectory;
const searchDirectoryMock = viewerApiMocks.searchDirectory;

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
        updatedAt: new Date("2023-10-21T11:00:00Z").toISOString(),
      },
      owner: { id: "owner-1", displayName: "DJ Nova" },
      profile: {},
      live: true,
      followerCount: 12,
    },
  ],
  generatedAt: new Date("2023-10-21T11:00:00Z").toISOString(),
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
        updatedAt: new Date("2023-10-21T12:30:00Z").toISOString(),
      },
      owner: { id: "owner-2", displayName: "PixelPro" },
      profile: {},
      live: true,
      followerCount: 8,
    },
  ],
  generatedAt: new Date("2023-10-21T12:30:00Z").toISOString(),
};

describe("BrowsePage", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    resetRouterMocks();
    setMockPathname("/browse");
  });

  test.each(directoryInputMatrix)("applies shared directory loading behavior for $label", async ({ query, normalized, mode, errorMessage }) => {
    const search = query ? `?q=${encodeURIComponent(query)}` : "";
    window.history.replaceState({}, "", `/browse${search}`);

    if (mode === "search") {
      searchDirectoryMock.mockResolvedValueOnce(searchDirectoryResponse as any);
    } else if (mode === "error") {
      searchDirectoryMock.mockRejectedValueOnce(new Error(errorMessage));
    } else {
      fetchDirectoryMock.mockResolvedValueOnce(baseDirectoryResponse as any);
    }

    render(<BrowsePage />);

    if (mode === "search") {
      await waitFor(() => expect(searchDirectoryMock).toHaveBeenCalledWith(normalized));
      expect((await screen.findAllByText("Retro Speedruns")).length).toBeGreaterThan(0);
      expect(fetchDirectoryMock).not.toHaveBeenCalled();
      return;
    }

    if (mode === "error") {
      await waitFor(() => expect(searchDirectoryMock).toHaveBeenCalledWith(normalized));
      expect(await screen.findByRole("alert")).toHaveTextContent(errorMessage);
      return;
    }

    await waitFor(() => expect(fetchDirectoryMock).toHaveBeenCalledTimes(1));
    expect((await screen.findAllByText("Deep Space Beats")).length).toBeGreaterThan(0);
    expect(searchDirectoryMock).not.toHaveBeenCalled();
  });

  test("uses the dedicated category param to load and focus an exact category view", async () => {
    window.history.replaceState({}, "", "/browse?category=Music");
    fetchDirectoryMock.mockResolvedValueOnce({
      channels: [
        ...baseDirectoryResponse.channels,
        {
          channel: {
            id: "chan-3",
            ownerId: "owner-3",
            title: "Checkpoint Central",
            category: "Gaming",
            tags: ["retro"],
            liveState: "live",
            currentSessionId: "session-3",
            createdAt: new Date("2023-10-19T10:00:00Z").toISOString(),
            updatedAt: new Date("2023-10-21T13:00:00Z").toISOString(),
          },
          owner: { id: "owner-3", displayName: "PadPro" },
          profile: {},
          live: true,
          followerCount: 6,
        },
      ],
      generatedAt: new Date("2023-10-21T13:00:00Z").toISOString(),
    } as any);

    render(<BrowsePage />);

    await waitFor(() => expect(fetchDirectoryMock).toHaveBeenCalledWith("Music"));
    expect((await screen.findAllByText("Deep Space Beats")).length).toBeGreaterThan(0);
    expect(screen.queryByText("Checkpoint Central")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Music" })).toHaveAttribute("aria-pressed", "true");
    expect(searchDirectoryMock).not.toHaveBeenCalled();
  });
});
