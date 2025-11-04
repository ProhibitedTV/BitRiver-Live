import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ChatPanel } from "../components/ChatPanel";
import { useAuth } from "../hooks/useAuth";
import { fetchChannelChat, sendChatMessage } from "../lib/viewer-api";
import type { ChatMessage } from "../lib/viewer-api";

jest.mock("../hooks/useAuth");

jest.mock("../lib/viewer-api", () => ({
  ...jest.requireActual("../lib/viewer-api"),
  fetchChannelChat: jest.fn(),
  sendChatMessage: jest.fn()
}));

const mockUseAuth = useAuth as jest.MockedFunction<typeof useAuth>;
const fetchChatMock = fetchChannelChat as jest.MockedFunction<typeof fetchChannelChat>;
const sendChatMock = sendChatMessage as jest.MockedFunction<typeof sendChatMessage>;

beforeEach(() => {
  jest.useFakeTimers();
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
});

afterEach(() => {
  jest.runOnlyPendingTimers();
  jest.useRealTimers();
});

test("renders chat history and sorts by time", async () => {
  const chatHistory: ChatMessage[] = [
    {
      id: "m2",
      message: "Later message",
      sentAt: new Date("2023-10-21T10:01:00Z").toISOString(),
      user: { id: "user-2", displayName: "Jax" }
    },
    {
      id: "m1",
      message: "Earlier message",
      sentAt: new Date("2023-10-21T10:00:00Z").toISOString(),
      user: { id: "user-1", displayName: "Rhea" }
    }
  ];
  fetchChatMock.mockResolvedValue(chatHistory);

  render(<ChatPanel channelId="chan-1" roomId="room-1" />);

  await waitFor(() => {
    expect(fetchChatMock).toHaveBeenCalledWith("chan-1");
    expect(screen.getByText("Earlier message")).toBeInTheDocument();
    expect(screen.getByText("Later message")).toBeInTheDocument();
  });

  const renderedMessages = screen.getAllByRole("listitem");
  expect(renderedMessages[0]).toHaveTextContent("Rhea");
  expect(renderedMessages[1]).toHaveTextContent("Jax");
});

test("sends a chat message when the user submits the form", async () => {
  const history: ChatMessage[] = [];
  fetchChatMock.mockResolvedValue(history);
  sendChatMock.mockResolvedValue({
    id: "m3",
    message: "Hello world",
    sentAt: new Date().toISOString(),
    user: { id: "viewer-1", displayName: "Viewer" }
  });

  const user = userEvent.setup({ advanceTimers: jest.advanceTimersByTime });
  render(<ChatPanel channelId="chan-99" roomId="room-1" />);

  const textarea = await screen.findByRole("textbox", { name: /chat message/i });
  expect(textarea).toHaveAttribute("placeholder", "Share your thoughts");
  expect(textarea).not.toBeDisabled();
  await user.type(textarea, "Hello world");
  const sendButton = screen.getByRole("button", { name: /send/i });
  await user.click(sendButton);

  await waitFor(() => {
    expect(sendChatMock).toHaveBeenCalledWith("chan-99", "viewer-1", "Hello world");
    expect(screen.getByText("Hello world")).toBeInTheDocument();
  });
});

test("treats unauthorized chat fetch as empty state for guests", async () => {
  const guestAuth = {
    user: undefined,
    loading: false,
    error: undefined,
    login: jest.fn(),
    signup: jest.fn(),
    logout: jest.fn(),
    refresh: jest.fn()
  };
  mockUseAuth.mockReturnValue(guestAuth as ReturnType<typeof useAuth>);
  fetchChatMock.mockRejectedValueOnce(new Error("401"));

  render(<ChatPanel channelId="chan-guest" roomId="room-1" />);

  await waitFor(() => {
    expect(fetchChatMock).toHaveBeenCalledWith("chan-guest");
    expect(screen.getByText(/no messages yet/i)).toBeInTheDocument();
  });

  expect(screen.queryByRole("alert")).not.toBeInTheDocument();

  jest.advanceTimersByTime(30_000);
  expect(fetchChatMock).toHaveBeenCalledTimes(1);

  const textarea = screen.getByRole("textbox", { name: /chat message/i });
  expect(textarea).toBeDisabled();
  expect(textarea).toHaveAttribute("placeholder", "Sign in to participate in chat");
});
