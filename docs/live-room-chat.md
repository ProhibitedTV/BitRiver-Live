# Live-room chat product contract

This document defines the target chat experience for BitRiver Live channel
pages. It translates the useful structure of classic social live rooms into
BitRiver Live's own restrained network-console identity.

This is an adapted BitRiver Live experience, **not an exact copy of ivlog.tv**.
Do not copy ivlog.tv branding, assets, icons, CSS, source code, or pixel-level
visual treatment. The product goal is a dense, legible room around a live
stream, implemented through BitRiver Live's existing chat stack.

## Product outcome

A channel page should feel like a shared live room rather than a video with a
comment box attached. Video remains the primary stage. Chat makes the active
audience, trusted room roles, stream state, and moderation state visible without
overwhelming playback.

The experience must remain useful when the room is quiet, busy, disconnected,
or being moderated. Visible state must come from real protocol state; the viewer
must not invent presence, authority, or stream status.

## Experience principles

- **Video first, room always near:** keep chat adjacent on wide screens and
  immediately below the player on narrow screens.
- **Dense, not cramped:** favor compact rows and predictable alignment while
  retaining readable wrapping, focus targets, and touch targets.
- **Authority is explicit:** owner, broadcaster, administrator, and moderator
  indicators come from server-authored metadata only.
- **Events explain the room:** stream and moderation transitions use distinct
  rows instead of looking like user-authored messages.
- **Quiet utility:** controls appear when relevant to the current user and
  action. Normal viewers do not receive an always-visible moderation console.
- **One chat stack:** extend `/api/chat/ws` and the existing REST history/report
  endpoints. Do not introduce a second WebSocket service, presence service, or
  frontend-only authority model.

## Responsive layout

### Desktop (900 px and wider)

- The player is the primary column; chat is a sticky right-side dock.
- The dock header contains room identity, live/offline state, viewer and chatter
  counts when known, connection state, and a compact options menu.
- The transcript owns the remaining vertical space. The composer stays at the
  bottom without covering messages.
- The roster is a compact rail, popover, or collapsible region inside the chat
  boundary. It must not reduce the player below a useful viewing width.
- At widths where the dock becomes too narrow, low-priority metadata collapses
  before message text or moderation feedback does.

### Tablet (640-899 px)

- Video stays first and chat follows in document order.
- The roster defaults collapsed and opens as an in-panel region or sheet.
- The composer remains reachable with the on-screen keyboard open.

### Mobile (below 640 px)

- Video remains the first major surface and chat follows directly below it.
- A nearby watch-navigation affordance may scroll to chat; chat must still be
  usable without a full-screen modal.
- Roster and moderation actions use disclosure controls with touch targets of
  at least 44 by 44 CSS pixels.
- No horizontal scrolling is required for message content, badges, timestamps,
  or actions. Long names and unbroken text wrap or truncate safely.

## Chat anatomy

### Header

Required information, in priority order:

1. Room/channel name.
2. Live or offline state, based on server state.
3. Connection state: live, reconnecting, polling fallback, or unavailable.
4. Chatter count and total viewer count when the backend can distinguish them.
5. Options for timestamps, avatars, roster visibility, and other non-critical
   presentation preferences.

### Message row

A normal row contains:

- optional compact timestamp;
- optional avatar or deterministic placeholder;
- server-authored badge slots in stable order;
- display name and normalized primary role;
- plain-text message content; and
- contextual report or moderation actions when authorized.

Consecutive messages from one author may be grouped for density, but every
message retains its own ID, timestamp, report target, and moderation target.
User content is always rendered as text. It must never be inserted as raw HTML.

### Room event row

Room events are visually and semantically separate from normal messages:

- **System:** live/offline transitions, pinned announcements, and room-mode
  changes.
- **Presence:** joins and leaves only when the operator enables them; high-volume
  rooms may show roster changes without transcript rows.
- **Moderation:** timeouts, timeout removal, bans, and unbans using a safe public
  summary. Private reasons remain limited to authorized viewers.
- **Automod:** an authorized moderation notice. Filter internals and rejected
  message content are not disclosed to ordinary viewers.
- **Error/local status:** send failure, reconnect state, or local transcript
  clearing. Local state must not masquerade as a server event.

## Presence and roster

The roster is driven by server `presence_*` events, not inferred from recent
messages.

- `presence_snapshot` replaces the current roster after a successful join.
- `presence_join` adds or refreshes one visible user.
- `presence_leave` removes one visible user.
- One authenticated user with multiple active connections appears once in the
  user list and chatter count.
- `viewerCount` is the room connection count; `chatterCount` is the de-duplicated
  authenticated user count. The UI labels them distinctly.
- The viewer sorts owner/broadcaster, administrator, moderator, then viewers;
  names sort deterministically within a role.
- Presence is ephemeral. It is neither transcript history nor an audit record.

If presence is unavailable during fallback polling, the viewer labels the
roster unavailable or shows known recent chatters as such. It must not label
recent chatters as currently online.

## Roles and badges

The server owns role and badge metadata. Clients may style known stable IDs but
must render unknown badges safely as text or omit them without failing.

| Role | Meaning | Default presentation |
| --- | --- | --- |
| `owner` | Channel owner for this room | Highest priority; Owner badge |
| `broadcaster` | Current broadcast identity when distinct from owner | Broadcaster badge |
| `admin` | Platform administrator | Admin badge; moderation controls |
| `moderator` | Room/platform moderator | Moderator badge; moderation controls |
| `viewer` | Normal participant | No required badge |

One user may have multiple badges. Authorization continues to use authenticated
server roles and channel ownership; a badge is display metadata, never an
authorization grant. Custom, paid, verified, or event badges are later schema
consumers of the existing stable badge slots, not new role types by default.

## Moderation and command model

Supported MVP commands map to existing authenticated gateway commands:

| Viewer input/action | Existing command | User-facing behavior |
| --- | --- | --- |
| Send text | `message` | Optimistic only when an acknowledgement can be reconciled; show failures |
| Timeout row action or `/timeout` | `timeout` | Require target and bounded duration; optional reason |
| Remove timeout or `/untimeout` | `remove_timeout` | Confirm success or show structured failure |
| Ban row action or `/ban` | `ban` | Deliberate destructive action with clear target |
| Unban row action or `/unban` | `unban` | Confirm success or show structured failure |
| Report message | REST report endpoint or `report` | Available to signed-in non-authors; preserve message ID |
| `/clear` | local viewer state only | Never claim room history was deleted |

The backend enforces every privileged command. Hiding a button is not an
authorization boundary. Unknown and unauthorized commands return actionable
errors without closing the socket.

`/me`, persistent message deletion, server-wide room clearing, slow mode,
followers-only mode, and `/title` are not current gateway capabilities. They
require explicit protocol, authorization, persistence/audit, and accessibility
decisions before the viewer exposes them.

## Accessibility requirements

- The transcript is a named `role="log"` with polite, additions-only live
  announcements. Loading skeletons and the entire panel are not live regions.
- Connection, send, report, and moderation outcomes use appropriate status or
  alert semantics without announcing the entire transcript again.
- All functions are keyboard reachable. Popovers and dialogs return focus to
  their trigger and support Escape.
- Text and state indicators meet WCAG 2.2 AA contrast. Color is never the only
  indication of role, live state, moderation state, or failure.
- Reduced-motion preferences disable non-essential animation.
- Message grouping preserves an accessible author and timestamp association for
  each individual message.
- Mobile disclosures have large touch targets and do not trap the on-screen
  keyboard over the composer.

## Busy-room performance

- Keep a bounded in-memory transcript. The current viewer limit of 500 entries
  is the MVP ceiling unless profiling justifies a different value.
- De-duplicate REST history, WebSocket broadcasts, and acknowledgements by
  stable message/event ID.
- Preserve a reader's scroll position. Auto-follow only when already near the
  bottom; otherwise show a “Jump to latest” affordance.
- Apply incremental presence updates rather than refetching the roster for each
  join or leave.
- Avoid per-message profile REST calls. Message and presence envelopes carry
  safe display metadata needed by the row.
- Coalesce or suppress noisy presence rows in busy rooms while keeping roster
  state current.
- Before increasing the transcript ceiling, profile render time and memory with
  sustained message bursts and long unbroken content.

## Feature-to-contract map

Status meanings:

- **Shipped:** wired through the current production path and covered by tests.
- **Partial:** a protocol or viewer primitive exists, but the visible end-to-end
  behavior is incomplete.
- **Required:** no complete production path exists yet.
- **Later:** intentionally outside the first complete live-room MVP.

| Visible feature | Current contract/source | Status and required change |
| --- | --- | --- |
| Desktop video plus right chat dock | `web/viewer/app/channels/[id]/page.tsx`, `styles/channel-watch.css` | **Shipped** under #1273. |
| Mobile video then chat | Channel watch document order and mobile navigation | **Shipped** under #1273. |
| Dense bounded transcript and safe text | `ChatPanel.tsx`, `MAX_MESSAGES`, React text rendering | **Shipped**; retain the 500-entry bound and plain-text rendering. |
| Scroll preservation and jump-to-latest | `ChatPanel.tsx`, chat-panel tests | **Shipped** under #1273. |
| Loading, empty, retry, auth, socket/fallback states | `ChatPanel.tsx` | **Shipped** under #1273. |
| Message role metadata | `message.user.role` in `internal/chat/PROTOCOL.md` | **Partial**: live messages carry it; REST history currently reconstructs display identity from `userId`. Preserve safe metadata in history responses. |
| Visible message badges | `message.user.badges`, `UserBadge` | **Partial**: payloads are parsed, but the viewer renders an empty reserved slot rather than actual badges. |
| Authoritative online roster | `presence_snapshot`, `presence_join`, `presence_leave` | **Partial**: gateway support and de-duplication shipped under #1274; the viewer currently shows recent chatters inferred from messages and does not consume presence envelopes. |
| Viewer versus chatter counts | `PresenceEvent.viewerCount/chatterCount` | **Partial**: protocol exists; viewer integration and fallback labeling are required. |
| System row presentation | `system` envelope and viewer room notices | **Partial**: the gateway and renderer exist, but production stream/pinned-notice services do not call `BroadcastSystemEvent`. |
| Moderation row actions and slash commands | `timeout`, `remove_timeout`, `ban`, `unban` | **Shipped** under #1275 with backend authorization. |
| Report action | Existing chat report REST path and viewer dialog | **Shipped** under #1275. |
| Moderation/automod audience policy | Room event broadcast and viewer room notices | **Partial**: define and enforce public-safe versus moderator-only payloads before exposing filter details or private reasons. |
| Stream live/offline notices | `SystemEvent` kinds can represent them | **Required**: wire real stream lifecycle transitions with deduplication. |
| Pinned announcement | `SystemEvent` can carry a notice | **Later**: add an authorized command/storage decision and current-pin snapshot behavior. |
| `/me`, message removal, room clear | Explicitly unsupported in `internal/chat/PROTOCOL.md` | **Later** after persistence and moderation-audit decisions. |
| Slow/followers-only mode and `/title` | No current gateway contract | **Later** after room-mode domain and authorization design. |

## Delivery boundary

### Complete live-room MVP

The first complete MVP consists of:

- the shipped responsive dock, transcript, composer, state handling, reports,
  and moderation commands from #1273 and #1275;
- the shipped backend presence/role/system event foundation from #1274;
- viewer consumption of authoritative presence and visible badge metadata;
- safe metadata parity between live messages and restored history;
- production wiring for live/offline system events; and
- an explicit audience policy for moderation and automod notices.

### Later extensions

Pinned announcements, custom badges, `/me`, persistent message removal, room
history clearing, slow/followers-only modes, and title changes follow only after
their domain, authorization, persistence, audit, and accessibility contracts
are approved.

## Architecture ownership

- `internal/domain` and `internal/service` own durable room-mode, announcement,
  stream-lifecycle, and moderation policy decisions.
- `internal/api` owns HTTP/WebSocket decoding and response translation.
- `internal/chat` remains the single realtime adapter for gateway fan-out,
  ephemeral presence, and chat event transport.
- `internal/storage` owns persistence adapters for transcript, moderation audit,
  or announcements when the domain contract requires persistence.
- `web/viewer` owns presentation, local input parsing, safe fallback behavior,
  focus management, and responsive layout. It never grants roles or invents
  authoritative presence.

New work must extend these boundaries and `internal/chat/PROTOCOL.md`. It must
not add a parallel chat backend, second WebSocket endpoint, browser-only room
authority, or a second deployment service.

## Issue sequence

Completed foundation:

- [#1273](https://github.com/ProhibitedTV/BitRiver-Live/issues/1273) — compact
  responsive viewer chat panel.
- [#1274](https://github.com/ProhibitedTV/BitRiver-Live/issues/1274) — presence,
  roles/badges, and system-event protocol foundation.
- [#1275](https://github.com/ProhibitedTV/BitRiver-Live/issues/1275) — viewer
  moderation actions and slash-command UX.

Remaining MVP:

- [#1382](https://github.com/ProhibitedTV/BitRiver-Live/issues/1382) — consume
  authoritative presence and render role/badge metadata with history parity.
- [#1383](https://github.com/ProhibitedTV/BitRiver-Live/issues/1383) — define and
  enforce the public-versus-privileged moderation event audience policy.
- [#1384](https://github.com/ProhibitedTV/BitRiver-Live/issues/1384) — publish
  de-duplicated stream live/offline notices from the production lifecycle path.

Implement remaining MVP work in that order. Pinned announcements, later
commands, and room modes require separate contracts and issues only when they
enter an active milestone; they are not implied by the MVP issue set above.
