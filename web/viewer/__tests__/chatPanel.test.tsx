import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ChatPanel } from "../components/ChatPanel";
import { useAuth } from "../hooks/useAuth";
import { fetchChannelChat, sendChatMessage } from "../lib/viewer-api";
import type { ChatTranscript } from "../lib/viewer-api";

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
  const transcript: ChatTranscript = {
    roomId: "room-1",
    participants: 3,
    messages: [
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
    ]
  };
  fetchChatMock.mockResolvedValue(transcript);

  render(<ChatPanel channelId="chan-1" roomId="room-1" />);

  await waitFor(() => {
    expect(fetchChatMock).toHaveBeenCalledWith("chan-1");
    expect(screen.getByText("Earlier message")).toBeInTheDocument();
    expect(screen.getByText("Later message")).toBeInTheDocument();
  });

  const messages = screen.getAllByRole("listitem");
  expect(messages[0]).toHaveTextContent("Rhea");
  expect(messages[1]).toHaveTextContent("Jax");
});

test("sends a chat message when the user submits the form", async () => {
  const transcript: ChatTranscript = {
    roomId: "room-1",
    participants: 1,
    messages: []
  };
  fetchChatMock.mockResolvedValue(transcript);
  sendChatMock.mockResolvedValue({
    id: "m3",
    message: "Hello world",
    sentAt: new Date().toISOString(),
    user: { id: "viewer-1", displayName: "Viewer" }
  });

  render(<ChatPanel channelId="chan-99" roomId="room-1" />);

  const textarea = await screen.findByRole("textbox", { name: /chat message/i });
  await userEvent.type(textarea, "Hello world");
  const sendButton = screen.getByRole("button", { name: /send/i });
  await userEvent.click(sendButton);

  await waitFor(() => {
    expect(sendChatMock).toHaveBeenCalledWith("chan-99", "Hello world");
    expect(screen.getByText("Hello world")).toBeInTheDocument();
  });
});
