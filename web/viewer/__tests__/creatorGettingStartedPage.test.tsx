import userEvent from "@testing-library/user-event";
import { screen, waitFor } from "@testing-library/react";
import {
  mockAnonymousUser,
  mockAuthenticatedUser,
  renderWithProviders,
  viewerApiMocks,
} from "../test/test-utils";
import CreatorGettingStartedPage from "../app/creator/getting-started/page";

jest.mock("../hooks/useAuth");

const fetchManagedChannelsMock = viewerApiMocks.fetchManagedChannels;
const fetchChannelPlaybackMock = viewerApiMocks.fetchChannelPlayback;
const originalViewerBasePath = process.env.NEXT_PUBLIC_VIEWER_BASE_PATH;

describe("CreatorGettingStartedPage", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    process.env.NEXT_PUBLIC_VIEWER_BASE_PATH = "/viewer";
    window.localStorage.clear();
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

  test("prompts guests to sign in", () => {
    mockAnonymousUser();

    renderWithProviders(<CreatorGettingStartedPage />);

    expect(screen.getByRole("button", { name: /sign in/i })).toBeInTheDocument();
  });

  test("supports channel selection and deep links for live setup and uploads", async () => {
    mockAuthenticatedUser({ id: "creator-1" });
    fetchManagedChannelsMock.mockResolvedValue([
      {
        id: "chan-1",
        ownerId: "creator-1",
        title: "Main",
        tags: [],
        liveState: "offline",
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
        streamKey: "abc",
        ingestEndpoints: ["rtmp://ingest"],
      },
      {
        id: "chan-2",
        ownerId: "creator-1",
        title: "Backup",
        tags: [],
        liveState: "offline",
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
        streamKey: "def",
        ingestEndpoints: ["rtmp://ingest2"],
      },
    ] as any);
    fetchChannelPlaybackMock.mockResolvedValue({ live: false, channel: { id: "chan-1" } } as any);

    const user = userEvent.setup();
    renderWithProviders(<CreatorGettingStartedPage />);

    await waitFor(() => expect(fetchManagedChannelsMock).toHaveBeenCalled());

    expect(screen.getByRole("link", { name: /open live setup/i })).toHaveAttribute("href", "/creator/live/chan-1");
    expect(screen.getByRole("link", { name: /open uploads/i })).toHaveAttribute("href", "/creator/uploads/chan-1");

    await user.selectOptions(screen.getByLabelText("Channel"), "chan-2");

    expect(screen.getByRole("link", { name: /open live setup/i })).toHaveAttribute("href", "/creator/live/chan-2");
    expect(screen.getByRole("link", { name: /open uploads/i })).toHaveAttribute("href", "/creator/uploads/chan-2");
  });

  test("marks go-live step complete from playback signal and allows manual sharing completion", async () => {
    mockAuthenticatedUser({ id: "creator-1" });
    fetchManagedChannelsMock.mockResolvedValue([
      {
        id: "chan-live",
        ownerId: "creator-1",
        title: "Live channel",
        tags: [],
        liveState: "live",
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
        streamKey: "stream-key",
        ingestEndpoints: ["rtmp://ingest"],
      },
    ] as any);
    fetchChannelPlaybackMock.mockResolvedValue({ live: true, channel: { id: "chan-live" } } as any);

    const user = userEvent.setup();
    renderWithProviders(<CreatorGettingStartedPage />);

    expect(await screen.findByText(/current status: live/i)).toBeInTheDocument();

    await user.click(screen.getByRole("checkbox", { name: /i shared my viewer link/i }));
    expect(screen.getByRole("checkbox", { name: /i shared my viewer link/i })).toBeChecked();
  });

  test("includes the configured viewer basePath when copying and linking to the public channel page", async () => {
    mockAuthenticatedUser({ id: "creator-1" });
    fetchManagedChannelsMock.mockResolvedValue([
      {
        id: "chan-share",
        ownerId: "creator-1",
        title: "Share channel",
        tags: [],
        liveState: "offline",
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
        streamKey: "stream-key",
        ingestEndpoints: ["rtmp://ingest"],
      },
    ] as any);
    fetchChannelPlaybackMock.mockResolvedValue({ live: false, channel: { id: "chan-share" } } as any);

    const user = userEvent.setup();
    renderWithProviders(<CreatorGettingStartedPage />);

    expect(await screen.findByRole("button", { name: /copy viewer link/i })).toBeEnabled();
    expect(screen.getByRole("link", { name: /preview viewer page/i })).toHaveAttribute("href", "/viewer/channels/chan-share");

    await user.click(screen.getByRole("button", { name: /copy viewer link/i }));
    expect(await screen.findByText(/viewer link copied/i)).toBeInTheDocument();
  });
});
