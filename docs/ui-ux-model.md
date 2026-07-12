# BitRiver Live UI/UX Model

## Purpose
This document defines the core UX model for BitRiver Live by clarifying who the primary users are, what success looks like for each user type, what they worry about, and which actions are mission-critical.

## User Roles

### 1) Operator (infrastructure + platform owner)

#### Primary goals
- Keep the platform available, stable, and secure.
- Deploy, configure, and upgrade BitRiver Live with predictable outcomes.
- Observe system health and quickly diagnose failures.
- Maintain compliance with internal operational requirements (backups, retention, access control, change management).

#### Primary fears
- Downtime during live events or peak traffic.
- Silent degradation (streams appear "up" but quality is poor or delayed).
- Misconfiguration that breaks ingest, playback, storage, or auth.
- Data loss (recordings, metadata, chat history, user/session state).
- Inability to recover quickly from incidents.

#### Critical actions they must be able to perform
- Bring up and tear down the full stack reliably using the canonical deployment path.
- Configure environment values and service integration points safely.
- Validate deployment/compose configuration before rollout.
- Monitor key service health indicators and logs.
- Perform backup/restore and disaster-recovery runbooks.
- Rotate credentials/secrets and apply upgrades with controlled risk.

---

### 2) Creator / Streamer

#### Primary goals
- Start a stream quickly with minimal setup friction.
- Deliver stable, high-quality live video/audio to viewers.
- Understand stream state in real time (live, reconnecting, degraded, offline).
- Manage stream lifecycle confidently before, during, and after broadcast.

#### Primary fears
- Going live fails unexpectedly right before or during a session.
- Quality drops (buffering, poor bitrate, latency spikes) hurt audience trust.
- Loss of control over what is live, recorded, or visible to viewers.
- Confusing status feedback that prevents timely troubleshooting.

#### Critical actions they must be able to perform
- Connect an encoder/ingest client and authenticate stream publishing.
- Start, stop, and restart streams without ambiguous state.
- Confirm live status and playback availability from a creator-facing perspective.
- Receive clear, actionable error and recovery guidance when ingest fails.
- Verify stream health cues (uptime, quality signals, reconnect state).

---

### 3) Viewer

#### Primary goals
- Find and play live content quickly.
- Watch streams with smooth playback and clear audio/video.
- Trust stream state (live vs ended) and experience consistent controls.
- Engage with content with minimal interruption.

#### Primary fears
- Playback fails, stalls, or starts too late.
- Stream status is misleading (appears live but is unavailable).
- Unexpected interruptions, poor quality, or excessive delay.
- UX friction across devices/browsers.

#### Critical actions they must be able to perform
- Open a stream page and start playback with low friction.
- Recover from transient playback failures via clear retry behavior.
- Understand whether a stream is live, starting, degraded, or offline.
- Use standard player controls reliably (play/pause, mute/volume, fullscreen).
- Continue session with predictable behavior across refresh/reconnect.

## Cross-role UX Principles
- **Clarity of state:** Every role should see explicit status instead of implicit assumptions.
- **Fast recovery:** Error paths should guide the next best action, not only report failure.
- **Operational transparency:** Operators and creators need enough context to debug without exposing unnecessary complexity to viewers.
- **Consistency:** Core actions and states should behave the same across environments and sessions.
- **Ecosystem identity:** Public surfaces use BitRiver's restrained network-console language, with gold structure, cyan live/focus state, and red failure state; visual status cues must always correspond to real state.
