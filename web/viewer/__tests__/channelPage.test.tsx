import userEvent from "@testing-library/user-event";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import ChannelPage from "../app/channels/[id]/page";
import { useAuth } from "../hooks/useAuth";
import {
  fetchChannelChat,
  fetchChannelPlayback,
  fetchChannelVods,
  followChannel,
  createTip,
  sendChatMessage,
  subscribeChannel,
  unfollowChannel,
  unsubscribeChannel
} from "../lib/viewer-api";

jest.mock("../hooks/useAuth");

jest.mock("../lib/viewer-api", () => ({
  ...jest.requireActual("../lib/viewer-api"),
  fetchChannelPlayback: jest.fn(),
  fetchChannelVods: jest.fn(),
  fetchChannelChat: jest.fn(),
  sendChatMessage: jest.fn(),
  followChannel: jest.fn(),
  unfollowChannel: jest.fn(),
  subscribeChannel: jest.fn(),
  unsubscribeChannel: jest.fn(),
  createTip: jest.fn()
}));

jest.mock("../components/Player", () => ({
  Player: () => <div data-testid="player" />
}));

const mockUseAuth = useAuth as jest.MockedFunction<typeof useAuth>;
const fetchChannelPlaybackMock = fetchChannelPlayback as jest.MockedFunction<typeof fetchChannelPlayback>;
const fetchChannelVodsMock = fetchChannelVods as jest.MockedFunction<typeof fetchChannelVods>;
const fetchChannelChatMock = fetchChannelChat as jest.MockedFunction<typeof fetchChannelChat>;
const sendChatMessageMock = sendChatMessage as jest.MockedFunction<typeof sendChatMessage>;
const followChannelMock = followChannel as jest.MockedFunction<typeof followChannel>;
const unfollowChannelMock = unfollowChannel as jest.MockedFunction<typeof unfollowChannel>;
const subscribeChannelMock = subscribeChannel as jest.MockedFunction<typeof subscribeChannel>;
const unsubscribeChannelMock = unsubscribeChannel as jest.MockedFunction<typeof unsubscribeChannel>;
const createTipMock = createTip as jest.MockedFunction<typeof createTip>;

const basePlaybackResponse = {
  channel: {
    id: "chan-42",
    ownerId: "owner-42",
    title: "Deep Space Beats",
    category: "Music",
    tags: ["lofi", "ambient"],
    liveState: "live",
    currentSessionId: "session-1",
    createdAt: new Date("2023-10-20T10:00:00Z").toISOString(),
    updatedAt: new Date("2023-10-21T11:00:00Z").toISOString()
  },
  owner: {
    id: "owner-42",
    displayName: "DJ Nova"
  },
  profile: {
    bio: "Streaming vinyl sets from a solar-powered cabin.",
    avatarUrl: undefined,
    bannerUrl: undefined
  },
  live: true,
  follow: {
    followers: 10,
    following: false
  },
  donationAddresses: [
    { currency: "eth", address: "0xabc123", note: "Main" },
    { currency: "btc", address: "bc1xyz" }
  ],
  subscription: {
    subscribers: 3,
    subscribed: false
  },
  playback: undefined,
  chat: {
    roomId: "room-1"
  }
};

const baseChatMessages = [
  {
    id: "msg-1",
    message: "Welcome to the stream!",
    sentAt: new Date("2023-10-21T12:00:00Z").toISOString(),
    user: {
      id: "owner-42",
      displayName: "DJ Nova",
      role: "host"
    }
  }
];

describe("ChannelPage", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    fetchChannelPlaybackMock.mockResolvedValue(basePlaybackResponse as any);
    fetchChannelVodsMock.mockResolvedValue({ channelId: "chan-42", items: [] } as any);
    fetchChannelChatMock.mockResolvedValue(baseChatMessages as any);
    sendChatMessageMock.mockResolvedValue({
      id: "msg-2",
      message: "Hello from viewer",
      sentAt: new Date("2023-10-21T12:05:00Z").toISOString(),
      user: {
        id: "viewer-1",
        displayName: "Viewer",
        role: "member"
      }
    } as any);
    followChannelMock.mockResolvedValue({ followers: 11, following: true } as any);
    unfollowChannelMock.mockResolvedValue({ followers: 10, following: false } as any);
    subscribeChannelMock.mockResolvedValue({ subscribers: 4, subscribed: true, tier: "Plus" } as any);
    unsubscribeChannelMock.mockResolvedValue({ subscribers: 3, subscribed: false } as any);
    createTipMock.mockResolvedValue({
      id: "tip-1",
      channelId: "chan-42",
      fromUserId: "viewer-1",
      amount: 5,
      currency: "ETH",
      provider: "viewer",
      reference: "txn-001",
      createdAt: new Date().toISOString()
    } as any);
  });

  test("renders playback details and supports follow, subscribe, and chat interactions", async () => {
    const user = userEvent.setup();
    mockUseAuth.mockReturnValue({
      user: { id: "viewer-1", displayName: "Viewer", email: "viewer@example.com", roles: [] },
      loading: false,
      error: undefined,
      login: jest.fn(),
      signup: jest.fn(),
      logout: jest.fn(),
      refresh: jest.fn()
    });

    render(<ChannelPage params={{ id: "chan-42" }} />);

    await waitFor(() => expect(fetchChannelPlaybackMock).toHaveBeenCalledWith("chan-42"));
    expect(await screen.findByRole("heading", { name: "Deep Space Beats" })).toBeInTheDocument();
    expect(screen.getByText(/streaming vinyl sets/i)).toBeInTheDocument();
    expect(await screen.findByText(/welcome to the stream/i)).toBeInTheDocument();

    const followButton = screen.getByRole("button", { name: /follow · 10 supporters/i });
    await user.click(followButton);
    await waitFor(() => expect(followChannelMock).toHaveBeenCalledWith("chan-42"));
    expect(screen.getByRole("button", { name: /following · 11 supporters/i })).toBeInTheDocument();
    expect(screen.getByText("11", { selector: "dd" })).toBeInTheDocument();

    const subscribeButton = screen.getByRole("button", { name: /subscribe/i });
    await user.click(subscribeButton);
    await waitFor(() => expect(subscribeChannelMock).toHaveBeenCalledWith("chan-42"));
    expect(screen.getByRole("button", { name: /subscribed · plus/i })).toBeInTheDocument();
    expect(screen.getByText("4", { selector: "dd" })).toBeInTheDocument();

    const textarea = screen.getByRole("textbox", { name: /chat message/i });
    await user.type(textarea, "Hello from viewer");
    await user.click(screen.getByRole("button", { name: "Send" }));

    await waitFor(() =>
      expect(sendChatMessageMock).toHaveBeenCalledWith("chan-42", "viewer-1", "Hello from viewer")
    );
    expect(await screen.findByText("Hello from viewer")).toBeInTheDocument();
  });

  test("refreshes follow and subscription state immediately after logging in", async () => {
    const authState = {
      user: undefined as
        | { id: string; displayName: string; email: string; roles: string[] }
        | undefined,
      loading: false,
      error: undefined,
      login: jest.fn(),
      signup: jest.fn(),
      logout: jest.fn(),
      refresh: jest.fn()
    };

    const initialResponse = {
      ...basePlaybackResponse,
      follow: { followers: 10, following: false },
      subscription: { subscribers: 3, subscribed: false }
    };

    const loggedInResponse = {
      ...basePlaybackResponse,
      follow: { followers: 11, following: true },
      subscription: { subscribers: 4, subscribed: true, tier: "Plus" }
    };

    fetchChannelPlaybackMock.mockResolvedValueOnce(initialResponse as any);
    fetchChannelPlaybackMock.mockResolvedValueOnce(loggedInResponse as any);
    fetchChannelPlaybackMock.mockResolvedValue(loggedInResponse as any);

    mockUseAuth.mockImplementation(() => authState);

    const { rerender } = render(<ChannelPage params={{ id: "chan-42" }} />);

    await waitFor(() => expect(fetchChannelPlaybackMock).toHaveBeenCalledTimes(1));

    expect(await screen.findByRole("button", { name: /follow · 10 supporters/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /subscribe/i })).toBeInTheDocument();

    await act(async () => {
      authState.user = {
        id: "viewer-1",
        displayName: "Viewer",
        email: "viewer@example.com",
        roles: []
      };
      rerender(<ChannelPage params={{ id: "chan-42" }} />);
    });

    await waitFor(() => expect(fetchChannelPlaybackMock).toHaveBeenCalledTimes(2));

    expect(
      await screen.findByRole("button", { name: /following · 11 supporters/i })
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /subscribed · plus/i })).toBeInTheDocument();
  });

  test("prompts authentication when the viewer is signed out", async () => {
    mockUseAuth.mockReturnValue({
      user: undefined,
      loading: false,
      error: undefined,
      login: jest.fn(),
      signup: jest.fn(),
      logout: jest.fn(),
      refresh: jest.fn()
    });

    render(<ChannelPage params={{ id: "chan-42" }} />);

    await waitFor(() => expect(fetchChannelPlaybackMock).toHaveBeenCalled());
    const followButton = await screen.findByRole("button", { name: /follow · 10 supporters/i });

    await act(async () => {
      followButton.click();
    });

    expect(followChannelMock).not.toHaveBeenCalled();
    expect(screen.getByText(/sign in from the header to follow this channel/i)).toBeInTheDocument();

    const subscribeButton = screen.getByRole("button", { name: /subscribe/i });
    await act(async () => {
      subscribeButton.click();
    });

    expect(subscribeChannelMock).not.toHaveBeenCalled();
    expect(screen.getByText(/sign in from the header to subscribe/i)).toBeInTheDocument();

    const textarea = await screen.findByRole("textbox", { name: /chat message/i });
    expect(textarea).toBeDisabled();
    expect(screen.getByRole("button", { name: "Send" })).toBeDisabled();

    const tipButton = screen.getByRole("button", { name: /send a tip/i });
    fireEvent.click(tipButton);
    expect(screen.getByText(/sign in from the header to send a tip/i)).toBeInTheDocument();
    expect(createTipMock).not.toHaveBeenCalled();
  });

  test("directs channel creators to the dashboard", async () => {
    mockUseAuth.mockReturnValue({
      user: { id: "owner-42", displayName: "DJ Nova", email: "nova@example.com", roles: [] },
      loading: false,
      error: undefined,
      login: jest.fn(),
      signup: jest.fn(),
      logout: jest.fn(),
      refresh: jest.fn(),
    });

    render(<ChannelPage params={{ id: "chan-42" }} />);

    await waitFor(() => expect(fetchChannelPlaybackMock).toHaveBeenCalledWith("chan-42"));

    const link = await screen.findByRole("link", { name: /open creator dashboard/i });
    expect(link).toHaveAttribute("href", "/creator/uploads/chan-42");
    expect(screen.getByText(/use your creator dashboard/i)).toBeInTheDocument();
  });

  test("surfaces VOD loading errors", async () => {
    mockUseAuth.mockReturnValue({
      user: { id: "viewer-1", displayName: "Viewer", email: "viewer@example.com", roles: [] },
      loading: false,
      error: undefined,
      login: jest.fn(),
      signup: jest.fn(),
      logout: jest.fn(),
      refresh: jest.fn()
    });

    fetchChannelVodsMock.mockRejectedValueOnce(new Error("VODs temporarily offline"));

    render(<ChannelPage params={{ id: "chan-42" }} />);

    await waitFor(() => expect(fetchChannelVodsMock).toHaveBeenCalledWith("chan-42"));

    expect(await screen.findByText(/couldn\'t load past broadcasts right now/i)).toBeInTheDocument();
    expect(screen.getByText(/VODs temporarily offline/i)).toBeInTheDocument();
  });
});
