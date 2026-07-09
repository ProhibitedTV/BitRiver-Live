## Scoped change: implement compact live-room chat panel (#1273)

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Establish live-room chat scope
  - Acceptance criteria:
    - `PLAN.md` captures #1273 scope, assumptions, risks, and test plan.
    - `TASKS.md` lists ordered tasks before source/doc edits for this pass.
    - Existing channel page layout, `ChatPanel`, chat API helpers, protocol docs, CSS, and tests are reviewed.

- [x] Task 2 - Refine chat panel structure and live-room header
  - Acceptance criteria:
    - Chat header shows room/channel identity, live/offline state, viewer count, message count, and compact sync state.
    - A compact roster/presence affordance is visible without consuming the transcript on mobile.
    - Secondary controls remain behind the options menu.

- [x] Task 3 - Add transcript row and scroll-follow behavior
  - Acceptance criteria:
    - Transcript preserves scroll position when the user is reading older messages.
    - Transcript auto-follows when the user is already at the bottom.
    - A jump-to-latest control appears when new messages arrive while the user is not at the bottom.
    - System/moderation row variants are visibly distinct from normal chat rows.

- [x] Task 4 - Update channel page styling and docs
  - Acceptance criteria:
    - Desktop keeps video primary with chat as a right-side dock.
    - Tablet/mobile stack keeps video first, chat reachable below the player, and composer pinned inside the panel.
    - Viewer README/chat contract notes reflect the live-room panel behavior if needed.

- [x] Task 5 - Verify and record results
  - Acceptance criteria:
    - Focused chat panel tests pass.
    - Channel page tests pass if affected.
    - Viewer lint passes or any host/tooling blocker is recorded.
    - `git diff --check` passes.

### Execution log
- Task 1 read-only pass:
  - Confirmed #1268 is closed after PR #1288; selected open issue #1273 as the next v1-visible product issue under #1272.
  - Reviewed `web/viewer/components/ChatPanel.tsx`, `web/viewer/__tests__/chatPanel.test.tsx`, `web/viewer/app/channels/[id]/page.tsx`, `web/viewer/lib/viewer-api-chat.ts`, `internal/chat/PROTOCOL.md`, `web/viewer/styles/chat.css`, `web/viewer/styles/channel-watch.css`, `web/viewer/styles/responsive.css`, and `web/viewer/README.md`.
  - Chose an incremental upgrade: preserve the existing REST/WebSocket/report/auth implementation while improving live-room identity, transcript behavior, roster affordance, system rows, and responsive layout.
- Task 2 implementation:
  - Updated `ChatPanel` header to show room identity, live/offline state, viewer count, message count, and compact sync state.
  - Added compact roster/presence affordance using available auth/chat users and viewer count.
  - Kept pop-out, sync detail, and display toggles behind the existing options menu.
  - Check: `npm.cmd --prefix web/viewer run test -- chatPanel.test.tsx --silent` passed.
- Task 3 implementation:
  - Added mixed transcript entries for user messages plus system/moderation notices.
  - Added scroll-follow behavior with a `Jump to latest` control when new messages arrive while the viewer is reading older chat.
  - Added focused tests for moderation rows and scroll preservation.
  - Check: `npm.cmd --prefix web/viewer run test -- chatPanel.test.tsx --silent` passed.
- Task 4 implementation:
  - Passed channel title, live state, and viewer count from the watch page into `ChatPanel`.
  - Adjusted chat panel, watch-page desktop grid, and responsive roster styling for a compact live-room dock.
  - Updated `web/viewer/README.md` chat contract with room identity, video-first layout, scroll-follow, and system/moderation row behavior.
  - Check: `npm.cmd --prefix web/viewer run test -- channelPage.test.tsx --silent` passed.
- Task 5 verification:
  - `npm.cmd --prefix web/viewer run test -- chatPanel.test.tsx --silent` passed.
  - `npm.cmd --prefix web/viewer run test -- channelPage.test.tsx --silent` passed.
  - `npm.cmd --prefix web/viewer run test -- --silent` hit a local Node heap OOM after `channelPage.test.tsx`; rerun with `NODE_OPTIONS=--max-old-space-size=4096` passed 25 suites / 208 tests.
  - `npm.cmd --prefix web/viewer run lint` passed with an existing warning in `components/UploadManager.tsx` about the `formValues` hook dependency.
  - `git diff --check` passed.
  - `& 'C:\Program Files\Git\bin\bash.exe' ./scripts/verify.sh` passed non-Docker stages through Compose config rendering and OME config validation, then failed at quickstart image build because the local Docker daemon pipe `npipe:////./pipe/dockerDesktopLinuxEngine` was unavailable. `com.docker.service` could not be started from this session; Docker Desktop launch did not make `docker version` responsive within the wait window.
