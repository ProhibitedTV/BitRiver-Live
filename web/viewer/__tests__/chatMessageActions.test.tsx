import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { mockUseAuth, signedInAuthState } from "../test/auth";
import { viewerApiMocks } from "../test/test-utils";
import { ChatPanel } from "../components/ChatPanel";

jest.mock("../hooks/useAuth");

const fetchChatMock = viewerApiMocks.fetchChannelChat;
const originalWebSocket = global.WebSocket;

class MockChatWebSocket {
  static instances: MockChatWebSocket[] = [];

  readyState = 0;
  sent: string[] = [];
  listeners: Record<string, Array<(event: any) => void>> = {};

  constructor(public url: string) {
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
  (global as any).WebSocket = MockChatWebSocket;
  mockUseAuth.mockReturnValue(signedInAuthState());
  fetchChatMock.mockResolvedValue([]);
});

afterEach(() => {
  jest.runOnlyPendingTimers();
  jest.useRealTimers();
  (global as any).WebSocket = originalWebSocket;
});

test("sends /me actions through the canonical message transport", async () => {
  const user = userEvent.setup({ advanceTimers: jest.advanceTimersByTime });
  render(<ChatPanel channelId="chan-action" roomId="room-action" />);

  await waitFor(() => expect(MockChatWebSocket.instances).toHaveLength(1));
  const socket = MockChatWebSocket.instances[0];
  await act(async () => {
    socket.open();
  });

  const textarea = await screen.findByRole("textbox", { name: /chat message/i });
  await user.type(textarea, "/me waves hello");
  await user.click(screen.getByRole("button", { name: /send/i }));

  expect(socket.sent.map((payload) => JSON.parse(payload))).toContainEqual({
    type: "message",
    channelId: "chan-action",
    content: "/me waves hello",
  });
  expect(screen.queryByText("Action messages are not supported yet.")).not.toBeInTheDocument();
  expect(textarea).toHaveValue("");
});

test("renders action content as plain text", async () => {
  render(<ChatPanel channelId="chan-safe-action" roomId="room-action" />);

  await waitFor(() => expect(MockChatWebSocket.instances).toHaveLength(1));
  const socket = MockChatWebSocket.instances[0];
  await act(async () => {
    socket.open();
    socket.receive({
      type: "event",
      event: {
        type: "message",
        occurredAt: "2026-09-13T02:20:00Z",
        message: {
          id: "action-1",
          channelId: "chan-safe-action",
          userId: "viewer-2",
          user: { id: "viewer-2", displayName: "Viewer Two", role: "viewer" },
          kind: "action",
          content: "* Viewer Two waves <b>hello</b>",
          createdAt: "2026-09-13T02:20:00Z",
        },
      },
    });
  });

  expect(await screen.findByText("* Viewer Two waves <b>hello</b>")).toBeInTheDocument();
  expect(document.querySelector(".chat-message__line b")).toBeNull();
});

test("removes a deleted message idempotently without rendering a delete notice", async () => {
  fetchChatMock.mockResolvedValue([
    {
      id: "delete-me",
      message: "remove this row",
      sentAt: "2026-09-13T02:20:00Z",
      user: { id: "viewer-2", displayName: "Viewer Two" },
    },
  ]);

  render(<ChatPanel channelId="chan-delete" roomId="room-delete" />);

  expect(await screen.findByText("remove this row")).toBeInTheDocument();
  await waitFor(() => expect(MockChatWebSocket.instances).toHaveLength(1));
  const socket = MockChatWebSocket.instances[0];
  await act(async () => {
    socket.open();
    const deletion = {
      type: "event",
      event: {
        type: "message_delete",
        occurredAt: "2026-09-13T02:21:00Z",
        messageDelete: {
          channelId: "chan-delete",
          messageId: "delete-me",
          deletedAt: "2026-09-13T02:21:00Z",
        },
      },
    };
    socket.receive(deletion);
    socket.receive(deletion);
  });

  await waitFor(() => expect(screen.queryByText("remove this row")).not.toBeInTheDocument());
  expect(screen.queryByText(/message delete/i)).not.toBeInTheDocument();
});
