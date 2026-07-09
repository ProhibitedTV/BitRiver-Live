## Scope (current change)
- Address GitHub issue #1273 by upgrading the existing Next.js `ChatPanel` into a compact live-room chat surface.
- Keep the channel video stage primary while making the chat dock feel like a live room: room identity, live/sync state, compact roster affordance, dense transcript, pinned composer.
- Reuse the existing REST history, `/api/chat/ws` WebSocket join/send path, auth handling, reports, and tests.
- Add scroll-follow behavior: auto-follow when the user is already near the bottom, preserve position when reading older messages, and expose a jump-to-latest control.
- Reserve visible row treatments for system/moderation events without implementing the full moderation console from #1275.
- Keep the implementation inside `web/viewer` and avoid deployment contract changes.
- Address CI failures found on PR #1289 without expanding runtime behavior:
  tighten the chat notice TypeScript guard and remove the transcoder live
  symlink test race that can occur when the stub process exits quickly.

## Assumptions
- The existing `ChatPanel` already satisfies core connection/send/report behavior; this pass should refine the live-room UX rather than introduce a second chat client.
- Backend role/presence metadata is not complete yet, so roster/badge UI should use available user/viewer count data and reserve slots for #1274.
- System/moderation rows can render from gateway event types and local connection notices, even if all backend event variants are not yet available.
- Viewer docs only need a short update if the chat contract changes visibly.

## Risks
- Chat UI can become bulky; keep controls compact and put secondary actions behind the existing options menu.
- Auto-scroll can annoy users reading older chat; only follow when already near the bottom and provide an explicit latest button otherwise.
- WebSocket event shapes may evolve; parse defensively and keep user content rendered as text, never HTML.
- Viewer test/build may be constrained by the host environment; use focused Jest tests first, then viewer lint/test if available.
- CI-only failures may come from stricter production TypeScript builds or test
  timing differences; keep fixes minimal and covered by focused checks.

## Test plan
- `npm.cmd --prefix web/viewer run test -- chatPanel.test.tsx`
- `npm.cmd --prefix web/viewer run test -- channelPage.test.tsx`
- `npm.cmd --prefix web/viewer run lint`
- `git diff --check`
- `npm.cmd --prefix web/viewer run build`
- `go test ./cmd/transcoder -run TestJobProducesSegmentsAndCanBeStopped -count=1`
- `npm.cmd --prefix web/viewer run test:playwright -- tests/channel-chat-playback.spec.ts tests/mobile-layout.spec.ts`
