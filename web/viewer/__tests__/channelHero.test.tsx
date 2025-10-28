import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { ChannelHero } from "../components/ChannelHero";
import { useAuth } from "../hooks/useAuth";
import {
  followChannel,
  subscribeChannel,
  unfollowChannel,
  unsubscribeChannel
} from "../lib/viewer-api";
import type { ChannelPlaybackResponse } from "../lib/viewer-api";

jest.mock("../hooks/useAuth");

jest.mock("../lib/viewer-api", () => ({
  ...jest.requireActual("../lib/viewer-api"),
  followChannel: jest.fn(),
  unfollowChannel: jest.fn(),
  subscribeChannel: jest.fn(),
  unsubscribeChannel: jest.fn()
}));

const mockUseAuth = useAuth as jest.MockedFunction<typeof useAuth>;
const followMock = followChannel as jest.MockedFunction<typeof followChannel>;
const unfollowMock = unfollowChannel as jest.MockedFunction<typeof unfollowChannel>;
const subscribeMock = subscribeChannel as jest.MockedFunction<typeof subscribeChannel>;
const unsubscribeMock = unsubscribeChannel as jest.MockedFunction<typeof unsubscribeChannel>;

const baseData: ChannelPlaybackResponse = {
  channel: {
    id: "chan-1",
    ownerId: "owner-1",
    title: "Deep Space Beats",
    category: "Music",
    tags: ["lofi", "relax"],
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
  follow: {
    followers: 10,
    following: false
  },
  subscription: {
    subscribed: false,
    subscribers: 4
  },
  playback: undefined,
  chat: { roomId: "room-1" }
};

beforeEach(() => {
  jest.clearAllMocks();
  mockUseAuth.mockReturnValue({
    user: { id: "viewer-1", displayName: "Viewer", email: "viewer@example.com", roles: [] },
    loading: false,
    error: undefined,
    login: jest.fn(),
    signup: jest.fn(),
    logout: jest.fn(),
    refresh: jest.fn()
  });

  followMock.mockResolvedValue({ followers: 11, following: true });
  unfollowMock.mockResolvedValue({ followers: 10, following: false });
  subscribeMock.mockResolvedValue({ subscribed: true, subscribers: 5 });
  unsubscribeMock.mockResolvedValue({ subscribed: false, subscribers: 4 });
});

test("renders follower and subscriber totals", () => {
  render(<ChannelHero data={baseData} />);

  expect(screen.getByText("Followers")).toBeInTheDocument();
  expect(screen.getByText("10", { selector: "dd" })).toBeInTheDocument();
  expect(screen.getByText("Subscribers")).toBeInTheDocument();
  expect(screen.getByText("4", { selector: "dd" })).toBeInTheDocument();
});

test("toggles follow and subscribe state", async () => {
  const onFollowChange = jest.fn();
  const onSubscriptionChange = jest.fn();

  render(
    <ChannelHero data={baseData} onFollowChange={onFollowChange} onSubscriptionChange={onSubscriptionChange} />
  );

  const followButton = screen.getByRole("button", { name: /follow · 10 supporters/i });
  fireEvent.click(followButton);

  await waitFor(() => {
    expect(followMock).toHaveBeenCalledWith("chan-1");
    expect(onFollowChange).toHaveBeenCalledWith({ followers: 11, following: true });
  });

  expect(screen.getByRole("button", { name: /following · 11 supporters/i })).toBeInTheDocument();

  const subscribeButton = screen.getByRole("button", { name: /subscribe/i });
  fireEvent.click(subscribeButton);

  await waitFor(() => {
    expect(subscribeMock).toHaveBeenCalledWith("chan-1");
    expect(onSubscriptionChange).toHaveBeenCalledWith({ subscribed: true, subscribers: 5 });
  });

  expect(screen.getByRole("button", { name: /subscribed/i })).toBeInTheDocument();
});
