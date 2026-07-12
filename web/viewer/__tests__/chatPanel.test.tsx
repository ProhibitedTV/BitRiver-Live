import { buildAuthUser, guestAuthState, mockUseAuth, signedInAuthState, viewerTwoUser } from "../test/auth";
import { viewerApiMocks } from "../test/test-utils";
import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ChatPanel } from "../components/ChatPanel";
import type { ChatMessage } from "../lib/viewer-api";

jest.mock("../hooks/useAuth");
const fetchChatMock = viewerApiMocks.fetchChannelChat;
const sendChatMock = viewerApiMocks.sendChatMessage;
const reportChatMock = viewerApiMocks.reportChatMessage;
const originalWebSocket = global.WebSocket;

class MockChatWebSocket {
  static instances: MockChatWebSocket[] = [];

  url: string;
  readyState = 0;
  sent: string[] = [];
  listeners: Record<string, Array<(event: any) => void>> = {};

  constructor(url: string) {
    this.url = url;
    MockChatWebSocket.instances.push(this);
  }

  addEventListener(type: string, listener: (event: any) => void) {
    this.listeners[type] = [...(this.listeners[type] ?? []), listener];
  }

  removeEventListener(type: string, listener: (event: any) => void) {
    this.listeners[type] = (this.listeners[type] ?? []).filter((candidate) => candidate !== listener);
  }

  send(payload: string) {
    this.sent.push(payload);
  }

  close() {
    this.readyState = 3;
    this.emit("close", {});
  }

  open() {
    this.readyState = 1;
    this.emit("open", {});
  }

  receive(payload: unknown) {
    this.emit("message", { data: JSON.stringify(payload) });
  }

  emit(type: string, event: unknown) {
    for (const listener of this.listeners[type] ?? []) {
      listener(event);
    }
  }
}

beforeEach(() => {
  jest.useFakeTimers({ legacyFakeTimers: true });
  jest.clearAllMocks();
  MockChatWebSocket.instances = [];
  (global as any).WebSocket = undefined;
  mockUseAuth.mockReturnValue(signedInAuthState());
});

afterEach(() => {
  jest.runOnlyPendingTimers();
  jest.useRealTimers();
  (global as any).WebSocket = originalWebSocket;
});

const advanceTimers = async (ms: number) => {
  await act(async () => {
    jest.advanceTimersByTime(ms);
  });
};

const runPendingTimers = async () => {
  await act(async () => {
    jest.runOnlyPendingTimers();
  });
};

const runAllTimers = async () => {
  await act(async () => {
    jest.runAllTimers();
  });
};

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

  const renderedMessages = within(screen.getByRole("log")).getAllByRole("listitem");
  expect(renderedMessages[0]).toHaveTextContent("Rhea");
  expect(renderedMessages[1]).toHaveTextContent("Jax");
});

test("renders compact live-room identity, counts, and roster", async () => {
  fetchChatMock.mockResolvedValue([
    {
      id: "m-room-1",
      message: "Welcome to the room",
      sentAt: new Date("2026-05-07T18:00:00Z").toISOString(),
      user: { id: "owner-42", displayName: "DJ Nova", role: "host" },
    },
    {
      id: "m-room-2",
      message: "Great set",
      sentAt: new Date("2026-05-07T18:01:00Z").toISOString(),
      user: { id: "viewer-2", displayName: "Viewer Two" },
    },
  ]);

  render(
    <ChatPanel
      channelId="chan-room"
      roomId="room-1"
      roomName="Deep Space Beats"
      live
      viewerCount={42}
    />
  );

  expect(await screen.findByRole("heading", { name: "Live chat" })).toBeInTheDocument();
  expect(screen.getByText("Deep Space Beats")).toBeInTheDocument();
  expect(await screen.findByText("Welcome to the room")).toBeInTheDocument();
  expect(screen.getByText("Live room")).toBeInTheDocument();
  expect(screen.getByText("42 viewers")).toBeInTheDocument();
  expect(screen.getByText("2 messages")).toBeInTheDocument();

  const roster = screen.getByLabelText("Room roster");
  expect(within(roster).getByText("42 viewers total")).toBeInTheDocument();
  expect(within(roster).getByText("Viewer")).toBeInTheDocument();
  expect(within(roster).getByText("DJ Nova")).toBeInTheDocument();
  expect(within(roster).getByText("host")).toBeInTheDocument();
});


test("scopes live announcements to the chat message log only", async () => {
  fetchChatMock.mockResolvedValue([]);

  render(<ChatPanel channelId="chan-live" roomId="room-1" />);

  await screen.findByText(/no messages yet/i);

  const panel = document.querySelector(".chat-panel");
  expect(panel).not.toBeNull();
  expect(panel).not.toHaveAttribute("aria-live");

  const body = document.querySelector(".chat-panel__body");
  expect(body).not.toBeNull();
  expect(body).not.toHaveAttribute("aria-live");

  expect(screen.queryByRole("log")).not.toBeInTheDocument();

  fetchChatMock.mockResolvedValueOnce([
    {
      id: "m-live-1",
      message: "Live message",
      sentAt: new Date().toISOString(),
      user: { id: "user-live", displayName: "Live User" }
    }
  ]);

  await advanceTimers(10_000);

  const log = await screen.findByRole("log");
  expect(log).toHaveAttribute("aria-live", "polite");
  expect(log).toHaveAttribute("aria-relevant", "additions text");
  expect(log).toHaveAttribute("aria-atomic", "false");
  expect(log.closest(".chat-panel__body")).toBeInTheDocument();

  const alert = screen.queryByRole("alert");
  if (alert) {
    expect(log).not.toContainElement(alert);
  }

  const status = screen.queryByRole("status");
  if (status) {
    expect(log).not.toContainElement(status);
  }
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

test("submits reports for another user's chat message", async () => {
  fetchChatMock.mockResolvedValue([
    {
      id: "m-report-1",
      message: "bad message",
      sentAt: new Date("2026-05-07T18:00:00Z").toISOString(),
      user: { id: "viewer-2", displayName: "Viewer Two" }
    }
  ]);
  reportChatMock.mockResolvedValue({
    id: "report-1",
    channelId: "chan-report",
    reporterId: "viewer-1",
    targetId: "viewer-2",
    reason: "Harassment",
    messageId: "m-report-1",
    status: "open",
    createdAt: "2026-05-07T18:01:00Z",
  });

  const user = userEvent.setup({ advanceTimers: jest.advanceTimersByTime });
  render(<ChatPanel channelId="chan-report" roomId="room-1" />);

  await screen.findByText("bad message");
  await user.click(screen.getByRole("button", { name: /report message from viewer two/i }));
  expect(screen.getByRole("dialog", { name: /report chat message/i })).toBeInTheDocument();
  await user.type(screen.getByRole("textbox", { name: /report reason/i }), "Harassment");
  await user.click(screen.getByRole("button", { name: "Submit report" }));

  await waitFor(() => {
    expect(reportChatMock).toHaveBeenCalledWith("chan-report", {
      targetId: "viewer-2",
      messageId: "m-report-1",
      reason: "Harassment",
    });
  });
  await waitFor(() => {
    expect(screen.getByRole("status")).toHaveTextContent("Report submitted for moderator review.");
  });
  expect(screen.queryByRole("dialog", { name: /report chat message/i })).not.toBeInTheDocument();
});

test("does not offer report controls on the current user's own messages", async () => {
  fetchChatMock.mockResolvedValue([
    {
      id: "m-own-1",
      message: "my own note",
      sentAt: new Date("2026-05-07T18:00:00Z").toISOString(),
      user: { id: "viewer-1", displayName: "Viewer" }
    }
  ]);

  render(<ChatPanel channelId="chan-own" roomId="room-1" />);

  await screen.findByText("my own note");
  expect(screen.queryByRole("button", { name: "Report" })).not.toBeInTheDocument();
});

test("joins live chat over websocket and renders inbound events without waiting for polling", async () => {
  (global as any).WebSocket = MockChatWebSocket;
  fetchChatMock.mockResolvedValue([]);

  render(<ChatPanel channelId="chan-ws" roomId="room-1" />);

  await waitFor(() => expect(fetchChatMock).toHaveBeenCalledWith("chan-ws"));
  await waitFor(() => expect(MockChatWebSocket.instances).toHaveLength(1));

  const socket = MockChatWebSocket.instances[0];
  expect(socket.url).toBe("ws://localhost/api/chat/ws");

  await act(async () => {
    socket.open();
  });

  expect(socket.sent[0]).toBe(JSON.stringify({ type: "join", channelId: "chan-ws" }));
  const user = userEvent.setup({ advanceTimers: jest.advanceTimersByTime });
  await user.click(screen.getByRole("button", { name: /open chat options/i }));
  expect(within(screen.getByLabelText("Chat options")).getByText("Live sync")).toBeVisible();

  await act(async () => {
    socket.receive({
      type: "event",
      event: {
        type: "message",
        message: {
          id: "m-ws-1",
          channelId: "chan-ws",
          userId: "viewer-2",
          content: "A live hello",
          createdAt: "2026-05-07T18:00:00Z",
        },
        occurredAt: "2026-05-07T18:00:00Z",
      },
    });
    socket.receive({
      type: "ack",
      event: {
        type: "message",
        message: {
          id: "m-ws-1",
          channelId: "chan-ws",
          userId: "viewer-2",
          content: "A live hello",
          createdAt: "2026-05-07T18:00:00Z",
        },
        occurredAt: "2026-05-07T18:00:00Z",
      },
    });
  });

  expect(screen.getAllByText("A live hello")).toHaveLength(1);
});

test("renders moderation events as system rows", async () => {
  (global as any).WebSocket = MockChatWebSocket;
  fetchChatMock.mockResolvedValue([]);

  render(<ChatPanel channelId="chan-mod" roomId="room-1" />);

  await waitFor(() => expect(MockChatWebSocket.instances).toHaveLength(1));
  const socket = MockChatWebSocket.instances[0];

  await act(async () => {
    socket.open();
    socket.receive({
      type: "event",
      event: {
        type: "automod",
        occurredAt: "2026-05-07T18:00:00Z",
        targetId: "viewer-2",
        reason: "link spam",
        filter: {
          name: "Blocked links",
          action: "hold",
        },
      },
    });
  });

  const log = await screen.findByRole("log");
  const notice = within(log).getByText("Automod").closest("li");
  expect(notice).toHaveClass("chat-system-row--moderation");
  expect(notice).toHaveTextContent("Filter: Blocked links");
  expect(notice).toHaveTextContent("Action: hold");
  expect(notice).toHaveTextContent("Target: viewer-2");
  expect(notice).toHaveTextContent("Reason: link spam");
});

test("preserves scroll position and offers jump to latest for background messages", async () => {
  (global as any).WebSocket = MockChatWebSocket;
  fetchChatMock.mockResolvedValue([]);

  render(<ChatPanel channelId="chan-scroll" roomId="room-1" />);

  await screen.findByText(/no messages yet/i);
  await waitFor(() => expect(MockChatWebSocket.instances).toHaveLength(1));
  const transcript = screen.getByTestId("chat-transcript-scroll");
  Object.defineProperty(transcript, "scrollHeight", { configurable: true, value: 1000 });
  Object.defineProperty(transcript, "clientHeight", { configurable: true, value: 200 });
  Object.defineProperty(transcript, "scrollTop", {
    configurable: true,
    writable: true,
    value: 100,
  });

  fireEvent.scroll(transcript);
  const socket = MockChatWebSocket.instances[0];

  await act(async () => {
    socket.open();
    socket.receive({
      type: "event",
      event: {
        type: "message",
        message: {
          id: "m-scroll-1",
          channelId: "chan-scroll",
          userId: "viewer-2",
          content: "Message below the fold",
          createdAt: "2026-05-07T18:00:00Z",
        },
        occurredAt: "2026-05-07T18:00:00Z",
      },
    });
  });

  expect(await screen.findByRole("button", { name: /jump to latest/i })).toBeInTheDocument();
  expect(transcript.scrollTop).toBe(100);

  fireEvent.click(screen.getByRole("button", { name: /jump to latest/i }));

  expect(transcript.scrollTop).toBe(1000);
  await waitFor(() => {
    expect(screen.queryByRole("button", { name: /jump to latest/i })).not.toBeInTheDocument();
  });
});

test("sends messages through the websocket when live chat is connected", async () => {
  (global as any).WebSocket = MockChatWebSocket;
  fetchChatMock.mockResolvedValue([]);

  const user = userEvent.setup({ advanceTimers: jest.advanceTimersByTime });
  render(<ChatPanel channelId="chan-send" roomId="room-1" />);

  await waitFor(() => expect(MockChatWebSocket.instances).toHaveLength(1));
  const socket = MockChatWebSocket.instances[0];

  await act(async () => {
    socket.open();
  });

  const textarea = await screen.findByRole("textbox", { name: /chat message/i });
  await user.type(textarea, "Live from socket");
  await user.click(screen.getByRole("button", { name: /send/i }));

  expect(sendChatMock).not.toHaveBeenCalled();
  expect(socket.sent).toContain(JSON.stringify({ type: "message", channelId: "chan-send", content: "Live from socket" }));
  expect(textarea).toHaveValue("");
});

test("sends slash moderation commands through the websocket", async () => {
  (global as any).WebSocket = MockChatWebSocket;
  mockUseAuth.mockReturnValue(signedInAuthState(buildAuthUser({ roles: ["admin"] })));
  fetchChatMock.mockResolvedValue([
    {
      id: "m-slash-1",
      message: "needs moderation",
      sentAt: new Date("2026-05-07T18:00:00Z").toISOString(),
      user: { id: "viewer-2", displayName: "Viewer Two" },
    },
  ]);

  const user = userEvent.setup({ advanceTimers: jest.advanceTimersByTime });
  render(<ChatPanel channelId="chan-slash" roomId="room-1" />);

  await screen.findByText("needs moderation");
  await waitFor(() => expect(MockChatWebSocket.instances).toHaveLength(1));
  const socket = MockChatWebSocket.instances[0];
  await act(async () => {
    socket.open();
  });

  const textarea = screen.getByRole("textbox", { name: /chat message/i });
  await user.type(textarea, "/timeout viewer-2 5m spam");
  await user.click(screen.getByRole("button", { name: /send/i }));

  const timeoutPayload = socket.sent.map((payload) => JSON.parse(payload)).find((payload) => payload.type === "timeout");
  expect(timeoutPayload).toEqual({
    type: "timeout",
    targetId: "viewer-2",
    durationMs: 300000,
    reason: "spam",
    channelId: "chan-slash",
  });
  expect(screen.getByRole("status")).toHaveTextContent("Timeout command sent for viewer-2.");
  expect(textarea).toHaveValue("");
});

test("clears the local transcript with the slash clear command", async () => {
  fetchChatMock.mockResolvedValue([
    {
      id: "m-clear-1",
      message: "clear this locally",
      sentAt: new Date("2026-05-07T18:00:00Z").toISOString(),
      user: { id: "viewer-2", displayName: "Viewer Two" },
    },
  ]);

  const user = userEvent.setup({ advanceTimers: jest.advanceTimersByTime });
  render(<ChatPanel channelId="chan-clear" roomId="room-1" />);

  await screen.findByText("clear this locally");
  await user.type(screen.getByRole("textbox", { name: /chat message/i }), "/clear");
  await user.click(screen.getByRole("button", { name: /send/i }));

  expect(screen.queryByText("clear this locally")).not.toBeInTheDocument();
  expect(screen.getByRole("status")).toHaveTextContent("Local chat view cleared.");
});

test("shows a local error when a viewer tries a moderation slash command", async () => {
  (global as any).WebSocket = MockChatWebSocket;
  fetchChatMock.mockResolvedValue([]);

  const user = userEvent.setup({ advanceTimers: jest.advanceTimersByTime });
  render(<ChatPanel channelId="chan-denied" roomId="room-1" />);

  await screen.findByText(/no messages yet/i);
  await waitFor(() => expect(MockChatWebSocket.instances).toHaveLength(1));
  await act(async () => {
    MockChatWebSocket.instances[0].open();
  });

  await user.type(screen.getByRole("textbox", { name: /chat message/i }), "/ban viewer-2");
  await user.click(screen.getByRole("button", { name: /send/i }));

  expect(screen.getByRole("alert")).toHaveTextContent("You do not have permission to moderate this room.");
  expect(MockChatWebSocket.instances[0].sent.map((payload) => JSON.parse(payload))).not.toContainEqual(
    expect.objectContaining({ type: "ban" })
  );
});

test("does not expose moderation actions to normal viewers", async () => {
  fetchChatMock.mockResolvedValue([
    {
      id: "m-viewer-actions-1",
      message: "viewer sees report only",
      sentAt: new Date("2026-05-07T18:00:00Z").toISOString(),
      user: { id: "viewer-2", displayName: "Viewer Two" },
    },
  ]);

  render(<ChatPanel channelId="chan-viewer-actions" channelOwnerId="owner-1" roomId="room-1" />);

  await screen.findByText("viewer sees report only");
  expect(screen.getByRole("button", { name: /report message from viewer two/i })).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: /timeout viewer two/i })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: /ban viewer two/i })).not.toBeInTheDocument();
});

test("exposes moderator row controls and sends moderation payloads", async () => {
  (global as any).WebSocket = MockChatWebSocket;
  mockUseAuth.mockReturnValue(signedInAuthState(buildAuthUser({ roles: ["moderator"] })));
  fetchChatMock.mockResolvedValue([
    {
      id: "m-mod-actions-1",
      message: "moderate this",
      sentAt: new Date("2026-05-07T18:00:00Z").toISOString(),
      user: { id: "viewer-2", displayName: "Viewer Two" },
    },
  ]);
  const confirmSpy = jest.spyOn(window, "confirm").mockReturnValue(true);

  const user = userEvent.setup({ advanceTimers: jest.advanceTimersByTime });
  render(<ChatPanel channelId="chan-mod-actions" roomId="room-1" />);

  await screen.findByText("moderate this");
  await waitFor(() => expect(MockChatWebSocket.instances).toHaveLength(1));
  const socket = MockChatWebSocket.instances[0];
  await act(async () => {
    socket.open();
  });

  await user.click(screen.getByRole("button", { name: /timeout viewer two for 5 minutes/i }));
  await user.click(screen.getByRole("button", { name: "Ban Viewer Two" }));

  const sentPayloads = socket.sent.map((payload) => JSON.parse(payload));
  expect(sentPayloads).toContainEqual({
    type: "timeout",
    targetId: "viewer-2",
    durationMs: 300000,
    reason: "Moderated from viewer chat",
    channelId: "chan-mod-actions",
  });
  expect(sentPayloads).toContainEqual({
    type: "ban",
    targetId: "viewer-2",
    reason: "Moderated from viewer chat",
    channelId: "chan-mod-actions",
  });
  expect(confirmSpy).toHaveBeenCalledWith("Ban Viewer Two from chat?");
  confirmSpy.mockRestore();
});

test("uses channel chat even when no room id is provided", async () => {
  fetchChatMock.mockResolvedValue([]);

  render(<ChatPanel channelId="chan-1" />);

  await waitFor(() => {
    expect(fetchChatMock).toHaveBeenCalledWith("chan-1");
    expect(screen.getByText(/no messages yet/i)).toBeInTheDocument();
  });

  const textarea = screen.getByRole("textbox", { name: /chat message/i });
  expect(textarea).not.toBeDisabled();
  expect(textarea).toHaveAttribute("placeholder", "Share your thoughts");
  expect(textarea).toHaveAttribute("aria-disabled", "false");

  const form = screen.getByRole("form", { name: /send a chat message/i });
  expect(form).not.toHaveAttribute("aria-disabled");
});

test("treats unauthorized chat fetch as empty state for guests", async () => {
  mockUseAuth.mockReturnValue(guestAuthState());
  fetchChatMock.mockRejectedValueOnce(new Error("401"));

  render(<ChatPanel channelId="chan-guest" roomId="room-1" />);

  await waitFor(() => {
    expect(fetchChatMock).toHaveBeenCalledWith("chan-guest");
    expect(screen.getByRole("button", { name: /sign in to chat/i })).toBeInTheDocument();
  });

  expect(screen.queryByRole("alert")).not.toBeInTheDocument();

  await advanceTimers(30_000);
  expect(fetchChatMock).toHaveBeenCalledTimes(1);

  expect(screen.queryByRole("textbox", { name: /chat message/i })).not.toBeInTheDocument();
  expect(screen.queryByText(/no messages yet/i)).not.toBeInTheDocument();
});

test("clears chat, shows sign-in prompt, and pauses polling on structured 401s", async () => {
  mockUseAuth.mockReturnValue(guestAuthState());
  fetchChatMock
    .mockResolvedValueOnce([
      {
        id: "m-structured-1",
        message: "Message before auth lapse",
        sentAt: new Date().toISOString(),
        user: { id: "user-structured", displayName: "Structured User" }
      }
    ])
    .mockRejectedValueOnce(
      new Error(JSON.stringify({ error: "Authentication required. Please login." }))
    );

  render(<ChatPanel channelId="chan-structured" roomId="room-1" />);

  await waitFor(() => {
    expect(fetchChatMock).toHaveBeenCalledWith("chan-structured");
    expect(screen.getByText("Message before auth lapse")).toBeInTheDocument();
  });

  await runAllTimers();

  await waitFor(() => {
    expect(fetchChatMock).toHaveBeenCalledTimes(2);
  });

  expect(screen.queryByText("Message before auth lapse")).not.toBeInTheDocument();
  expect(
    screen.getByText("Sign in to view and participate in chat.")
  ).toBeInTheDocument();

  await runAllTimers();
  expect(fetchChatMock).toHaveBeenCalledTimes(2);

  expect(screen.queryByRole("textbox", { name: /chat message/i })).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: /sign in to chat/i })).toBeInTheDocument();
});

test("backs off after consecutive server errors and shows retry surface", async () => {
  fetchChatMock.mockRejectedValue(new Error("500"));

  render(<ChatPanel channelId="chan-error" roomId="room-1" />);

  await waitFor(() => {
    expect(fetchChatMock).toHaveBeenCalledTimes(1);
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Unable to load chat. We'll retry in a bit."
    );
  });

  await advanceTimers(19_999);
  expect(fetchChatMock).toHaveBeenCalledTimes(1);
  await advanceTimers(1);
  await waitFor(() => expect(fetchChatMock).toHaveBeenCalledTimes(2));

  await advanceTimers(39_999);
  expect(fetchChatMock).toHaveBeenCalledTimes(2);
  await advanceTimers(1);
  await waitFor(() => expect(fetchChatMock).toHaveBeenCalledTimes(3));

  await advanceTimers(59_999);
  expect(fetchChatMock).toHaveBeenCalledTimes(3);
  await advanceTimers(1);
  await waitFor(() => expect(fetchChatMock).toHaveBeenCalledTimes(4));

  await advanceTimers(59_999);
  expect(fetchChatMock).toHaveBeenCalledTimes(4);
  await advanceTimers(1);
  await waitFor(() => expect(fetchChatMock).toHaveBeenCalledTimes(5));

  expect(screen.getByRole("alert")).toHaveTextContent(
    "Unable to load chat. We'll retry in a bit."
  );
});

test("resumes chat polling once a guest signs in", async () => {
  mockUseAuth.mockReturnValue(guestAuthState());
  fetchChatMock.mockRejectedValueOnce(new Error("401"));

  const { rerender } = render(<ChatPanel channelId="chan-auth" roomId="room-1" />);

  await waitFor(() => {
    expect(fetchChatMock).toHaveBeenCalledWith("chan-auth");
    expect(screen.getByRole("button", { name: /sign in to chat/i })).toBeInTheDocument();
  });
  expect(fetchChatMock).toHaveBeenCalledTimes(1);

  mockUseAuth.mockReturnValue(signedInAuthState(viewerTwoUser));
  fetchChatMock.mockResolvedValueOnce([
    {
      id: "m-auth-1",
      message: "Welcome back",
      sentAt: new Date().toISOString(),
      user: { id: "viewer-2", displayName: "Viewer Two" }
    }
  ]);

  rerender(<ChatPanel channelId="chan-auth" roomId="room-1" />);

  await waitFor(() => {
    expect(fetchChatMock).toHaveBeenCalledTimes(2);
    expect(screen.getByText("Welcome back")).toBeInTheDocument();
  });

  const textarea = screen.getByRole("textbox", { name: /chat message/i });
  expect(textarea).not.toBeDisabled();
  expect(textarea).toHaveAttribute("placeholder", "Share your thoughts");
});

test("chat options menu exposes secondary actions and closes on Escape", async () => {
  fetchChatMock.mockResolvedValue([]);
  const user = userEvent.setup({ advanceTimers: jest.advanceTimersByTime });

  render(<ChatPanel channelId="chan-dialogs" roomId="room-1" />);

  await screen.findByText(/no messages yet/i);

  const trigger = screen.getByRole("button", { name: /open chat options/i });
  await user.click(trigger);
  const menu = screen.getByLabelText("Chat options");
  expect(menu).toBeVisible();
  expect(within(menu).getByRole("button", { name: /open pop-out chat/i })).toBeVisible();
  expect(within(menu).getByRole("checkbox", { name: /toggle chat avatars/i })).toBeChecked();
  expect(within(menu).getByRole("checkbox", { name: /toggle chat message timestamps/i })).toBeChecked();

  await user.keyboard("{Escape}");
  await waitFor(() => {
    expect(menu).not.toBeVisible();
  });
  expect(trigger).toHaveFocus();
});

test("chat options menu toggles display settings without a modal", async () => {
  fetchChatMock.mockResolvedValue([]);
  const user = userEvent.setup({ advanceTimers: jest.advanceTimersByTime });

  render(<ChatPanel channelId="chan-focus" roomId="room-1" />);

  await screen.findByText(/no messages yet/i);
  await user.click(screen.getByRole("button", { name: /open chat options/i }));
  const timestampsToggle = screen.getByRole("checkbox", { name: /toggle chat message timestamps/i });
  await user.click(timestampsToggle);
  expect(timestampsToggle).not.toBeChecked();
  expect(screen.queryByRole("dialog", { name: /chat settings/i })).not.toBeInTheDocument();
});

test("opens pop-out chat directly from the options menu", async () => {
  fetchChatMock.mockResolvedValue([]);
  const user = userEvent.setup({ advanceTimers: jest.advanceTimersByTime });
  const openSpy = jest.spyOn(window, "open").mockImplementation(() => null);

  render(<ChatPanel channelId="chan-return" roomId="room-1" />);

  await screen.findByText(/no messages yet/i);

  const trigger = screen.getByRole("button", { name: /open chat options/i });
  await user.click(trigger);
  await user.click(screen.getByRole("button", { name: /open pop-out chat/i }));

  expect(openSpy).toHaveBeenCalledWith(expect.any(String), "bitriver-chat-popout", "width=420,height=720,noopener,noreferrer");
  expect(screen.getByLabelText("Chat options")).not.toBeVisible();
  expect(trigger).toHaveFocus();
  openSpy.mockRestore();
});

test("escape closes the report dialog", async () => {
  fetchChatMock.mockResolvedValue([
    {
      id: "m-dialog-report",
      message: "needs review",
      sentAt: new Date("2026-05-07T18:00:00Z").toISOString(),
      user: { id: "viewer-2", displayName: "Viewer Two" }
    }
  ]);
  const user = userEvent.setup({ advanceTimers: jest.advanceTimersByTime });

  render(<ChatPanel channelId="chan-exclusive" roomId="room-1" />);

  await screen.findByText("needs review");
  await user.click(screen.getByRole("button", { name: /report message from viewer two/i }));
  expect(screen.getByRole("dialog", { name: /report chat message/i })).toBeInTheDocument();

  await user.keyboard("{Escape}");
  await waitFor(() => {
    expect(screen.queryByRole("dialog", { name: /report chat message/i })).not.toBeInTheDocument();
  });
});
