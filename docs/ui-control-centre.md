# Control Centre UX Audit and Redesign Proposal

## Scope and constraints
This document audits the current Control Centre UX implemented in `web/static/index.html` + `web/static/app.js` and proposes a redesign focused on operator clarity and trust.

Constraints applied:
- No backend capabilities are assumed beyond what current APIs already expose.
- Any UI that would require backend additions is explicitly marked as a dependency.
- Preference is given to fewer, clearer screens.

---

## 1) UX audit of the current Control Centre

## What exists today (as implemented)
- Top-level navigation has many peers: `Dashboard`, `Users`, `Channels`, `Go Live`, `Chat`, `Moderation`, `Legal`, `Profiles`, `Sessions`, `Uploads`, `Analytics`, `Settings`.
- “Overview” (`Dashboard`) includes:
  - A system status card populated from `/api/status`.
  - General business/platform metric cards (users, channels, live channels, streaming hours, chat messages, etc.).
- Stream control/lifecycle information is split across:
  - `Channels` (shows `liveState` at channel level).
  - `Go Live` (start/stop controls and `liveState`).
  - `Sessions` (historical start/end/duration/peak/renditions).

## Focus area audit findings

### A. System status visibility (ingest, transcoder, playback readiness, degraded vs healthy)
**Strengths**
- `/api/status` output is surfaced and includes per-component checks, remediation text, last checked time, and recent failures.
- Status badge semantics (ready/degraded/down/disabled) are already used consistently.
- “Log hints” are present and copyable, which helps actionability.

**Gaps**
- Health is visually present but not hierarchically prioritized over secondary metrics.
- Ingest/transcoder checks are shown as a flat list; there is no “pipeline” view to explain how one failure impacts playback readiness.
- Playback readiness is not explicitly represented as a first-class operator concept on the Overview; operators infer it from mixed signals.
- “Degraded” and “down” are visible as badge colors/text, but impact severity and blast radius are not explicit (for example: “new streams blocked” vs “existing streams may continue”).

**Trust impact**
- Operators can see statuses, but must mentally compose what they mean operationally. This increases cognitive load and slows incident response.

### B. Stream lifecycle visibility (state, last state change + reason)
**Strengths**
- Current channel `liveState` is visible in both Channels and Go Live.
- Session history shows start/end times and duration.

**Gaps**
- Lifecycle is fragmented across pages rather than unified per stream/channel.
- There is no explicit “last state change” field for channel lifecycle in the Control Centre.
- There is no explicit “reason for state change” field (e.g., operator stop, ingest failure, dependency failure).
- Requested lifecycle vocabulary (`idle / ingesting / live / degraded / ended`) is not fully represented by currently exposed UI data.

**Backend dependency note**
- `idle / ingesting / live / degraded / ended` as a unified state machine and “last state change + reason” are not available as a single existing payload in current UI code. A clear backend state summary endpoint or enriched channel/session payload is required for complete implementation.

### C. Failure communication (what broke, ownership, recommended action)
**Strengths**
- `detail` + `remediation` exist per status check.
- `recentFailures` gives a quick degraded-component list.

**Gaps**
- Ownership is implied by component names but not explicit in UI language (“Owned by: SRS/OME/transcoder/config”).
- Failures are listed as rows; they do not use an incident-oriented structure: problem, owner, impact, action, verification.
- Recommended actions are present but not ranked as “next best action,” and there is no explicit success criteria check (“resolved when status becomes ready”).

**Trust impact**
- In incidents, operators need certainty: what is broken, who owns it, what to do now. Current format has data, but not enough decision framing.

---

## 2) Proposed navigation structure (fewer screens, clearer hierarchy)

## Design principle
Use 3 operational screens as the core trust loop:
1. **Overview** (is the platform safe to operate?)
2. **Streams** (what is each stream lifecycle state?)
3. **System Health** (deep dive into component failures and remediation)

Keep existing product/admin surfaces (users, moderation, analytics, etc.) but move them behind an “Administration” grouping so operations-critical tasks are not diluted.

## Proposed primary nav
1. **Overview**
2. **Streams**
3. **System Health**
4. **Administration** (group): Users, Channels config, Chat, Moderation, Legal, Profiles, Uploads, Analytics, Settings

## Page structure
- **Overview dashboard** (default landing)
  - Global operational summary + active incidents + top streams.
- **Stream detail page** (from Streams list)
  - Single-channel lifecycle timeline + runtime state + stream controls.
- **System health page**
  - Full component matrix, ownership, remediation, and logs.

This reduces context switching and gives operators a stable path:
**Overview → Stream detail (if stream issue) or System health (if infrastructure issue).**

---

## 3) Wireframe-style page descriptions

## A) Overview dashboard

## Information architecture
1. **Header band: Global state**
   - `Overall status` badge (`ready/degraded/down/disabled` from `/api/status`).
   - `Last checked` timestamp.
   - Primary CTA: `Refresh status`.

2. **Critical pipeline status strip**
   - Three cards: **Ingest**, **Transcoder**, **Playback readiness**.
   - Each card shows:
     - Status badge
     - One-line human meaning (e.g., “Ingest healthy; new streams can connect”).
     - If non-ready: `Owner` + `Next action`.

3. **Active failures panel (incident-first)**
   - For each active failure from `recentFailures`:
     - **What is broken**: component + detail
     - **Owned by**: mapped owner label (SRS / OME / transcoder / config / platform)
     - **Operator action**: remediation text
     - **Verify recovery**: “Recheck status and confirm component returns to ready.”

4. **Streams at a glance table**
   - Columns: Channel, Current state, Started/Updated, Health indicator.
   - Row click opens **Stream detail**.

5. **Secondary metrics (collapsed or lower priority)**
   - Existing overview cards (users/channels/hours/messages/etc.) retained but visually demoted below operational health.

## Data mapping and dependencies
- Can be implemented now with existing `/api/status`, channel list, and sessions.
- **Dependency for full fidelity:** explicit “playback readiness” boolean/status is not currently a dedicated field; deriving it from existing checks may be approximate.

---

## B) Stream detail page

## Purpose
Single source of truth for one channel’s lifecycle and operational readiness.

## Layout
1. **Top summary card**
   - Channel name, owner, stream key actions (existing functionality).
   - **Current lifecycle state** badge.

2. **Lifecycle timeline card**
   - “Current state”
   - “Last state change time”
   - “Last state change reason”
   - “Previous state”

3. **Live pipeline card (channel-scoped where possible)**
   - Ingest status
   - Transcoder status
   - Playback status
   - Explicit warning if any dependency degraded.

4. **Operator actions card**
   - Start stream / Stop stream / Rotate key.
   - Safe-action helper text (what each action affects).

5. **Session history card**
   - Existing session records (start/end/duration/peak/renditions).

## Data mapping and dependencies
- Existing UI can render current `liveState` and session history now.
- **Backend dependency required** for:
  - Canonical lifecycle states (`idle / ingesting / live / degraded / ended`) as one stream-state model.
  - `last state change` timestamp and `reason`.
  - Channel-scoped ownership attribution for failure reason where applicable.

---

## C) System health page

## Purpose
Deep diagnostic page for operators during degraded/down conditions.

## Layout
1. **Status matrix**
   - Table columns:
     - Component
     - Category (ingest/core)
     - Status
     - Detail
     - Owned by
     - Last checked

2. **Failure communication panel (selected component)**
   - **What is broken** (plain language)
   - **Ownership** (SRS / OME / transcoder / config / platform)
   - **Recommended action** (from remediation)
   - **Runbook/log command** (copy button from log hints)

3. **Degraded vs healthy semantics legend**
   - Ready = normal operation
   - Degraded = partial impact, service still reachable
   - Down = hard failure requiring intervention
   - Disabled = intentionally not in use

4. **Recovery confirmation checklist**
   - Re-run status refresh
   - Confirm component `ready`
   - Confirm stream playback test if relevant

## Data mapping and dependencies
- Mostly implementable from existing `/api/status` payload.
- **Optional backend dependency** if desired: explicit impact metadata per component (e.g., “blocks new ingest”, “affects playback only”). Today this would be inferred in frontend logic.

---

## Implementation notes (UX-only)
- Prefer persistent, plain-language labels over abbreviations in incident contexts.
- Treat remediation as “next action” text, not hidden metadata.
- Keep color + icon + text together for status (do not rely on color alone).
- Promote “last updated” timestamps to improve trust in data freshness.

## Suggested rollout order
1. Restructure navigation and Overview prioritization (frontend only).
2. Add Stream detail page shell using existing fields.
3. Add ownership labeling and incident framing to System health.
4. Add backend-backed lifecycle metadata (state change + reason) when API support is ready.
