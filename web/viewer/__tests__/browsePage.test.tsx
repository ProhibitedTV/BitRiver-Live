import userEvent from "@testing-library/user-event";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
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
    window.history.replaceState({}, "", "/browse");
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

  it("hydrates topic drill-ins from the URL and keeps the primary grid actionable", async () => {
    window.history.replaceState({}, "", "/browse?topic=Music");
    fetchDirectoryMock.mockResolvedValueOnce(baseDirectoryResponse as any);

    render(<BrowsePage />);

    await waitFor(() => expect(fetchDirectoryMock).toHaveBeenCalledTimes(1));
    expect(await screen.findByRole("button", { name: "Music" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getAllByText("Music - 1 channel").length).toBeGreaterThan(0);
    const heading = screen.getByRole("heading", { level: 3, name: "Deep Space Beats" });
    const card = heading.closest("article");
    expect(card).toBeTruthy();
    expect(within(card!).getByRole("link", { name: "Open channel" })).toHaveAttribute("href", "/channels/chan-1");
  });

  it("pushes the selected topic into the browse URL when a filter chip is clicked", async () => {
    const user = userEvent.setup();
    fetchDirectoryMock.mockResolvedValueOnce(baseDirectoryResponse as any);

    render(<BrowsePage />);

    expect((await screen.findAllByText("Deep Space Beats")).length).toBeGreaterThan(0);
    await user.click(screen.getByRole("button", { name: "Music" }));

    await waitFor(() => expect(window.location.search).toBe("?topic=Music"));
    await waitFor(() => expect(screen.getByRole("button", { name: "Music" })).toHaveAttribute("aria-pressed", "true"));
  });

  it("keeps discovered filters visible after search narrows the result set", async () => {
    const user = userEvent.setup();
    fetchDirectoryMock.mockResolvedValueOnce(baseDirectoryResponse as any);
    searchDirectoryMock.mockResolvedValueOnce(searchDirectoryResponse as any);

    render(<BrowsePage />);

    expect((await screen.findAllByText("Deep Space Beats")).length).toBeGreaterThan(0);
    expect(screen.getByRole("button", { name: "Music" })).toBeVisible();

    fireEvent.change(screen.getByRole("searchbox", { name: /search channels/i }), { target: { value: "retro" } });
    await user.click(screen.getByRole("button", { name: /search/i }));

    await waitFor(() => expect(searchDirectoryMock).toHaveBeenCalledWith("retro"));
    expect((await screen.findAllByText("Retro Speedruns")).length).toBeGreaterThan(0);
    expect(screen.getByRole("button", { name: "Music" })).toBeVisible();
    expect(screen.getByRole("button", { name: "Gaming" })).toBeVisible();
  });

  it("clears query, topic, and sort state from the compact reset control", async () => {
    const user = userEvent.setup();
    window.history.replaceState({}, "", "/browse?q=retro&topic=Gaming");
    searchDirectoryMock.mockResolvedValueOnce(searchDirectoryResponse as any);

    render(<BrowsePage />);

    expect((await screen.findAllByText("Retro Speedruns")).length).toBeGreaterThan(0);
    await user.click(screen.getByRole("tab", { name: "New" }));
    await user.click(screen.getByRole("button", { name: "Reset" }));

    expect(window.location.pathname + window.location.search).toBe("/browse");
    await waitFor(() => expect(screen.getByRole("tab", { name: "Live" })).toHaveAttribute("aria-selected", "true"));
  });
});
