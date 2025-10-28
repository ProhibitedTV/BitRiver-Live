# Chat Gateway Protocol

The chat gateway exposes a WebSocket endpoint under `/api/chat/ws`. The
connection **must** be established with a valid BitRiver Live session cookie; the
existing session middleware attaches the authenticated user to the request
context prior to the upgrade.

## Client messages

All messages are JSON encoded. The following commands are available:

| Type             | Required fields                             | Description |
| ---------------- | ------------------------------------------- | ----------- |
| `join`           | `channelId`                                  | Subscribe the connection to a channel room. Must be called before sending chat or moderation commands. |
| `leave`          | `channelId`                                  | Unsubscribe from the room. |
| `message`        | `channelId`, `content`                       | Submit a chat message on behalf of the authenticated user. |
| `timeout`        | `channelId`, `targetId`, `durationMs`        | Issue a timeout (in milliseconds) against another user. Only channel owners and admins are allowed to moderate. |
| `remove_timeout` | `channelId`, `targetId`                      | Clear an active timeout. |
| `ban`            | `channelId`, `targetId`                      | Ban a user from joining chat. |
| `unban`          | `channelId`, `targetId`                      | Lift a previously issued ban. |

Unknown commands yield an `error` response without closing the connection.

## Server messages

Responses from the server also use JSON:

- `{"type":"ack","event":<Event>}` confirms a command that generated an
  immediate result (for example, posting a chat message).
- `{"type":"event","event":<Event>}` broadcasts chat message and moderation
  events to all clients subscribed to the affected channel.
- `{"type":"error","error":"..."}` reports validation failures or rejected
  commands.

The `<Event>` object mirrors the Go `chat.Event` structure and always carries an
`occurredAt` timestamp. Message events include the message ID, author and the
UTC creation time so that clients can update their transcripts without a REST
roundtrip.

## Lightweight JS client

A minimal browser-friendly client lives in `/web/static/chat-client.js` and exposes
helpers to:

- establish the WebSocket connection with automatic re-connects,
- join/leave channel rooms,
- emit chat messages and moderation commands, and
- register callbacks for inbound events and errors.

The admin dashboard (`app.js`) consumes this helper, but the viewer UI can reuse
the same surface to display live chat alongside the broadcast.
