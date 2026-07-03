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
const createChannelMock = viewerApiMocks.createChannel;
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
    expect(screen.getByRole("button", { name: /create account/i })).toBeInTheDocument();
  });

  test("lets a signed-in viewer create a first channel and unlock live setup", async () => {
    mockAuthenticatedUser({ id: "viewer-1", roles: [] });
    fetchManagedChannelsMock.mockResolvedValue([] as any);
    createChannelMock.mockResolvedValue({
      id: "chan-new",
      ownerId: "viewer-1",
      title: "My First Channel",
      category: "Just Chatting",
      tags: ["launch"],
      liveState: "offline",
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
      streamKey: "sk-first",
      ingestEndpoints: ["rtmp://ingest"],
    } as any);

    const user = userEvent.setup();
    renderWithProviders(<CreatorGettingStartedPage />);

    expect(await screen.findByRole("heading", { level: 3, name: /create your first channel/i })).toBeInTheDocument();
    expect(screen.getByText(/creator tools unlock after the first channel is created/i)).toBeInTheDocument();

    await user.type(screen.getByLabelText("Channel name"), "My First Channel");
    await user.type(screen.getByLabelText("Primary category"), "Just Chatting");
    await user.type(screen.getByLabelText("Tags"), "launch");
    await user.click(screen.getByRole("button", { name: /create channel/i }));

    await waitFor(() => {
      expect(createChannelMock).toHaveBeenCalledWith({
        title: "My First Channel",
        category: "Just Chatting",
        tags: ["launch"],
      });
    });

    expect(await screen.findByText(/channel created\. your live setup is ready below\./i)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /open live setup/i })).toHaveAttribute("href", "/creator/live/chan-new");
    expect(screen.getByRole("button", { name: /copy viewer link/i })).toBeEnabled();
  });

  test("supports channel selection and keeps the live setup links in sync", async () => {
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

    expect(screen.getAllByRole("link", { name: /open live setup/i })).toHaveLength(1);
    for (const link of screen.getAllByRole("link", { name: /open live setup/i })) {
      expect(link).toHaveAttribute("href", "/creator/live/chan-1");
    }
    expect(screen.getByRole("link", { name: /open public preview/i })).toHaveAttribute("href", "/viewer/channels/chan-1");
    expect(screen.getByRole("link", { name: /preview viewer page/i })).toHaveAttribute("href", "/viewer/channels/chan-1");

    await user.selectOptions(screen.getByLabelText("Channel"), "chan-2");

    for (const link of screen.getAllByRole("link", { name: /open live setup/i })) {
      expect(link).toHaveAttribute("href", "/creator/live/chan-2");
    }
    expect(screen.getByRole("link", { name: /open public preview/i })).toHaveAttribute("href", "/viewer/channels/chan-2");
    expect(screen.getByRole("link", { name: /preview viewer page/i })).toHaveAttribute("href", "/viewer/channels/chan-2");
  });

  test("surfaces live status directly once playback is active", async () => {
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
    expect(screen.getByText("Live")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /copy viewer link/i }));
    expect(await screen.findByText(/viewer link copied/i)).toBeInTheDocument();
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
