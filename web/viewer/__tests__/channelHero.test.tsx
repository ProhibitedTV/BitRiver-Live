import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { ChannelAboutPanel, ChannelHeader } from "../components/ChannelHero";
import { useAuth } from "../hooks/useAuth";
import {
  followChannel,
  createTip,
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
  unsubscribeChannel: jest.fn(),
  createTip: jest.fn()
}));

const mockUseAuth = useAuth as jest.MockedFunction<typeof useAuth>;
const followMock = followChannel as jest.MockedFunction<typeof followChannel>;
const unfollowMock = unfollowChannel as jest.MockedFunction<typeof unfollowChannel>;
const subscribeMock = subscribeChannel as jest.MockedFunction<typeof subscribeChannel>;
const unsubscribeMock = unsubscribeChannel as jest.MockedFunction<typeof unsubscribeChannel>;
const createTipMock = createTip as jest.MockedFunction<typeof createTip>;

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
  donationAddresses: [
    { currency: "eth", address: "0xabc123", note: "Main wallet" },
    { currency: "btc", address: "bc1xyz" }
  ],
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
  createTipMock.mockResolvedValue({
    id: "tip-1",
    channelId: "chan-1",
    fromUserId: "viewer-1",
    amount: 5,
    currency: "ETH",
    provider: "viewer",
    reference: "txn-001",
    walletAddress: "0xabc123",
    createdAt: new Date().toISOString()
  } as any);
});

afterEach(() => {
  // Clean up clipboard overrides between tests.
  delete (navigator as unknown as Record<string, unknown>).clipboard;
});

test("renders follower and subscriber totals", () => {
  render(<ChannelHeader data={baseData} />);

  expect(screen.getByText("Followers")).toBeInTheDocument();
  expect(screen.getByText("10", { selector: "dd" })).toBeInTheDocument();
  expect(screen.getByText("Subscribers")).toBeInTheDocument();
  expect(screen.getByText("4", { selector: "dd" })).toBeInTheDocument();
});

test("renders QR codes for each donation address", () => {
  render(<ChannelAboutPanel data={baseData} />);

  const qrCodes = screen.getAllByRole("img", { name: /address qr code/i });
  expect(qrCodes).toHaveLength(baseData.donationAddresses?.length ?? 0);
  expect(screen.getByRole("img", { name: /eth address qr code/i })).toBeInTheDocument();
  expect(screen.getByRole("img", { name: /btc address qr code/i })).toBeInTheDocument();
});

test("toggles follow and subscribe state", async () => {
  const onFollowChange = jest.fn();
  const onSubscriptionChange = jest.fn();

  render(
    <ChannelHeader
      data={baseData}
      onFollowChange={onFollowChange}
      onSubscriptionChange={onSubscriptionChange}
    />
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

test("disables follow actions for channel owners", () => {
  mockUseAuth.mockReturnValue({
    user: { id: "owner-1", displayName: "DJ Nova", email: "dj@nova.fm", roles: [] },
    loading: false,
    error: undefined,
    login: jest.fn(),
    signup: jest.fn(),
    logout: jest.fn(),
    refresh: jest.fn()
  });

  render(<ChannelHeader data={baseData} />);

  const followButton = screen.getByRole("button", { name: /follow · 10 supporters/i });
  expect(followButton).toBeDisabled();

  fireEvent.click(followButton);

  expect(followMock).not.toHaveBeenCalled();
  expect(unfollowMock).not.toHaveBeenCalled();
  expect(
    screen.getByText(/you manage this channel\. followers will see your updates here\./i)
  ).toBeInTheDocument();
});

test("allows viewers to send a tip and surfaces confirmation", async () => {
  render(<ChannelHeader data={baseData} />);

  fireEvent.click(screen.getByRole("button", { name: /send a tip/i }));

  const dialog = await screen.findByRole("dialog", { name: /send a tip/i });
  const amountInput = within(dialog).getByLabelText(/amount/i);
  fireEvent.change(amountInput, { target: { value: "7.5" } });

  const referenceInput = within(dialog).getByLabelText(/wallet reference/i);
  fireEvent.change(referenceInput, { target: { value: "hash-42" } });

  const messageInput = within(dialog).getByLabelText(/message/i);
  fireEvent.change(messageInput, { target: { value: "Keep the beats flowing" } });

  fireEvent.click(within(dialog).getByRole("button", { name: /send tip/i }));

  await waitFor(() => {
    expect(createTipMock).toHaveBeenCalledWith("chan-1", {
      amount: 7.5,
      currency: "ETH",
      provider: "viewer",
      reference: "hash-42",
      walletAddress: "0xabc123",
      message: "Keep the beats flowing"
    });
  });

  await waitFor(() => {
    expect(screen.getByText(/thanks for supporting deep space beats/i)).toBeInTheDocument();
  });

  await waitFor(() => {
    expect(screen.queryByRole("dialog", { name: /send a tip/i })).not.toBeInTheDocument();
  });
});

test("validates tip details before calling the API", async () => {
  render(<ChannelHeader data={baseData} />);

  fireEvent.click(screen.getByRole("button", { name: /send a tip/i }));
  const dialog = await screen.findByRole("dialog", { name: /send a tip/i });

  const amountInput = within(dialog).getByLabelText(/amount/i);
  fireEvent.change(amountInput, { target: { value: "0" } });
  const form = dialog.querySelector("form");
  expect(form).toBeTruthy();
  fireEvent.submit(form as HTMLFormElement);

  const validationAlert = await within(dialog).findByRole("alert");
  expect(validationAlert).toHaveTextContent(/enter a valid amount greater than zero/i);
  expect(createTipMock).not.toHaveBeenCalled();

  fireEvent.change(amountInput, { target: { value: "5" } });
  fireEvent.submit(form as HTMLFormElement);

  const referenceAlert = await within(dialog).findByRole("alert");
  expect(referenceAlert).toHaveTextContent(/provide the wallet or transaction reference/i);
  expect(createTipMock).not.toHaveBeenCalled();
});

test("surfaces tip submission errors", async () => {
  createTipMock.mockRejectedValueOnce(new Error("Payment provider unavailable"));

  render(<ChannelHeader data={baseData} />);

  fireEvent.click(screen.getByRole("button", { name: /send a tip/i }));
  const dialog = await screen.findByRole("dialog", { name: /send a tip/i });

  fireEvent.change(within(dialog).getByLabelText(/amount/i), { target: { value: "5" } });
  fireEvent.change(within(dialog).getByLabelText(/wallet reference/i), { target: { value: "abc" } });

  fireEvent.click(within(dialog).getByRole("button", { name: /send tip/i }));

  expect(
    await within(dialog).findByText(/payment provider unavailable/i)
  ).toBeInTheDocument();
  expect(createTipMock).toHaveBeenCalledTimes(1);
  expect(screen.getByRole("dialog", { name: /send a tip/i })).toBeInTheDocument();
});

test("renders donation addresses and copies to clipboard", async () => {
  const writeText = jest.fn().mockResolvedValue(undefined);
  Object.defineProperty(navigator, "clipboard", {
    configurable: true,
    value: { writeText }
  });

  render(<ChannelAboutPanel data={baseData} />);

  expect(screen.getByRole("heading", { name: /support this channel/i })).toBeInTheDocument();
  const donationItems = screen.getAllByRole("listitem");
  expect(donationItems).toHaveLength(2);
  const firstDonation = donationItems[0];
  expect(within(firstDonation).getAllByText(/^ETH$/)).not.toHaveLength(0);
  expect(within(firstDonation).getByText(/Main wallet/i)).toBeInTheDocument();
  expect(within(firstDonation).getByText(/0xabc123/i)).toBeInTheDocument();
  const secondDonation = donationItems[1];
  expect(within(secondDonation).getAllByText(/^BTC$/)).not.toHaveLength(0);
  expect(within(secondDonation).getByText(/bc1xyz/i)).toBeInTheDocument();

  const copyButton = screen.getByRole("button", { name: /copy eth address/i });
  fireEvent.click(copyButton);

  await waitFor(() => {
    expect(writeText).toHaveBeenCalledWith("0xabc123");
  });

  expect(
    screen.getByText(/ETH address copied to clipboard\./i)
  ).toBeInTheDocument();
});
