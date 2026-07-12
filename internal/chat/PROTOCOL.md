# Chat Gateway Protocol

The chat gateway exposes a WebSocket endpoint under `/api/chat/ws`. The
connection **must** be established with a valid BitRiver Live session cookie; the
existing session middleware attaches the authenticated user to the request
context prior to the upgrade.

## Client messages

All messages are JSON encoded. The following commands are available:

| Type             | Required fields                              | Description |
| ---------------- | -------------------------------------------- | ----------- |
| `join`           | `channelId`                                  | Subscribe the connection to a channel room. Must be called before sending chat or moderation commands. |
| `leave`          | `channelId`                                  | Unsubscribe this connection from the room. |
| `message`        | `channelId`, `content`                       | Submit a chat message on behalf of the authenticated user. |
| `timeout`        | `channelId`, `targetId`, `durationMs`        | Issue a timeout in milliseconds against another user. Only channel owners and admins are allowed to moderate. |
| `remove_timeout` | `channelId`, `targetId`                      | Clear an active timeout. |
| `ban`            | `channelId`, `targetId`                      | Ban a user from joining chat. |
| `unban`          | `channelId`, `targetId`                      | Lift a previously issued ban. |
| `report`         | `channelId`, `targetId`, `reason`            | Submit a moderation report. `messageId` and `evidenceUrl` are optional. |

Unknown commands yield an `error` response without closing the connection.

## Server envelopes

Responses from the server also use JSON:

- `{"type":"ack","event":<Event>}` confirms a command that generated an
  immediate result. Join acknowledgements omit `event`.
- `{"type":"event","event":<Event>}` broadcasts room events to subscribed
  clients.
- `{"type":"error","error":"..."}` reports validation failures or rejected
  commands.

The `<Event>` object always carries `type` and `occurredAt`. Existing clients
that only understand `message`, `moderation`, `report`, or `automod` can ignore
new event types.

## Event shapes

### Message

`message` events carry the persisted transcript payload plus optional safe
author metadata:

```json
{
  "type": "message",
  "occurredAt": "2026-07-09T17:30:00Z",
  "message": {
    "id": "msg_123",
    "channelId": "channel_123",
    "userId": "user_123",
    "user": {
      "id": "user_123",
      "displayName": "RiverFan",
      "role": "viewer",
      "badges": [{ "id": "broadcaster", "label": "Broadcaster" }]
    },
    "content": "hello room",
    "createdAt": "2026-07-09T17:30:00Z"
  }
}
```

`message.user` is additive. Older clients can continue using `message.userId`.
Roles are normalized to one of `owner`, `admin`, `moderator`, `broadcaster`, or
`viewer` with stable badge IDs for compact rendering.

### Presence

After a successful `join`, the joining connection receives a
`presence_snapshot` event:

```json
{
  "type": "presence_snapshot",
  "occurredAt": "2026-07-09T17:30:00Z",
  "presence": {
    "channelId": "channel_123",
    "users": [
      {
        "id": "user_123",
        "displayName": "RiverFan",
        "role": "viewer",
        "badges": []
      }
    ],
    "viewerCount": 2,
    "chatterCount": 1
  }
}
```

When the first active connection for a user joins a room, other room clients
receive `presence_join`. When that user's final active connection leaves,
remaining room clients receive `presence_leave`:

```json
{
  "type": "presence_join",
  "occurredAt": "2026-07-09T17:30:00Z",
  "presence": {
    "channelId": "channel_123",
    "user": {
      "id": "user_456",
      "displayName": "ModOne",
      "role": "moderator",
      "badges": [{ "id": "moderator", "label": "Moderator" }]
    },
    "viewerCount": 3,
    "chatterCount": 2
  }
}
```

`viewerCount` is the active WebSocket connection count for the chat room.
`chatterCount` is the de-duplicated authenticated user count. A user with
multiple tabs counts once in `users` and `chatterCount`.

Presence is live transport state only. It is not persisted as transcript
history and should not be used as an audit record.

### System Events

Backend services can broadcast room notices with `system` events:

```json
{
  "type": "system",
  "occurredAt": "2026-07-09T17:30:00Z",
  "system": {
    "id": "notice_123",
    "channelId": "channel_123",
    "kind": "stream_live",
    "message": "Stream went live.",
    "actorId": "user_owner",
    "targetId": "user_viewer",
    "actor": {
      "id": "user_owner",
      "displayName": "Creator",
      "role": "owner",
      "badges": [
        { "id": "owner", "label": "Owner" },
        { "id": "broadcaster", "label": "Broadcaster" }
      ]
    },
    "target": {
      "id": "user_viewer",
      "displayName": "Viewer",
      "role": "viewer"
    },
    "metadata": { "state": "live" },
    "createdAt": "2026-07-09T17:30:00Z"
  }
}
```

System events are for live room notices such as stream state changes, pinned
announcements, room mode changes, and moderation summaries. They are not
persisted by the chat worker unless a future storage contract explicitly adds an
audit destination.

### Moderation, reports, and automod

`moderation`, `report`, and `automod` event payloads retain their existing
shape. Moderation actions still use the existing commands: `timeout`,
`remove_timeout`, `ban`, and `unban`.

## Lightweight JS client

A minimal browser-friendly client lives in `/web/static/chat-client.js` and
exposes helpers to:

- establish the WebSocket connection with automatic re-connects,
- join/leave channel rooms,
- emit chat messages and moderation commands, and
- register callbacks for inbound events and errors.

The admin dashboard (`app.js`) consumes this helper, and the viewer UI uses the
same server envelopes for live chat alongside the broadcast.
