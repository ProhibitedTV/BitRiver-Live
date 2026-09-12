# Viewer browser and playback compatibility

This document defines the browser/device support boundary for the BitRiver Live v1.2.3 single-host release line. It is intentionally narrower than “works in every modern browser.” A browser class is supported only when the release-critical Playwright matrix and the documented media qualification agree.

## Release-blocking browser matrix

The viewer CI runs the complete existing regression suite on desktop Chromium and a focused release-critical suite on every browser/device class below. Playwright retries are disabled; a browser-specific failure must be fixed or explicitly removed from the support boundary rather than retried into green.

| Browser/device class | CI project | Qualification level | Notes |
| --- | --- | --- | --- |
| Chromium desktop | `chromium-regression` + `chromium-compat` | Supported | Full viewer regression plus the release-critical compatibility flow. |
| Firefox desktop | `firefox-compat` | Supported viewer shell; HLS capability-gated | The bundled Linux Firefox used by Playwright may expose neither an hls.js-usable MSE codec path nor native HLS. CI requires a visible playback-unavailable fallback in that case while chat, auth, accessibility, creator setup, and recovery remain functional. Shipping Firefox playback still depends on the browser/OS media capabilities described below. |
| WebKit desktop | `webkit-compat` | Supported WebKit engine class | Playwright WebKit is a Safari-equivalent engine check, **not evidence of testing a particular shipping Safari build on Apple hardware**. |
| Android/Chrome mobile | `android-chrome-compat` | Supported emulated mobile class | Playwright Pixel 7 profile with touch/mobile behavior. This is browser/device emulation, not a physical Android-device certification. |
| iPhone/WebKit mobile | `iphone-webkit-compat` | Supported emulated mobile class | Playwright iPhone 15/WebKit profile with touch/mobile behavior. This is not physical iPhone/Safari certification. |

The release-critical suite covers:

- channel watch/player initialization;
- HLS source attachment, native-HLS fallback, or an explicit unsupported-playback state when neither capability exists;
- chat history, send, and authentication fallback;
- transient playback/API recovery;
- signed-out navigation and the in-viewer authentication surface;
- WCAG A/AA axe checks on a critical public flow;
- creator live-setup rendering and server-authored ingest/stream-key state;
- mobile touch navigation and horizontal-overflow protection.

## Playback support boundary

### HLS and LL-HLS

HLS is the broad compatibility path. The viewer uses `hls.js` when Media Source Extensions are available. When `hls.js` is unavailable but the browser advertises native `application/vnd.apple.mpegurl` playback, the viewer assigns the manifest directly to the `<video>` element. The API may label the same broader-compatibility path as `ll-hls` with low-latency hints.

The compatibility suite proves that the viewer initializes the HLS path when the tested engine exposes a usable hls.js or native-HLS capability; otherwise it must leave loading and render an explicit unavailable state. It also covers transient playback API recovery across each declared engine class. It does **not** replace the release candidate's real-media golden path, which is responsible for proving playlist advancement and decoded media through the actual SRS/transcoder/OvenMediaEngine stack.

### WebRTC

The viewer has an OvenPlayer-backed WebRTC path when the API returns `protocol: "webrtc"`. WebRTC is **not promoted to a browser-wide support claim by the Playwright compatibility matrix alone**. NAT/ICE/TURN behavior, hardware/browser media behavior, and the real OME path require production-like media qualification. Operators must not infer physical-device WebRTC support from the mocked compatibility tests.

### Autoplay and audio

BitRiver Live does not promise audible autoplay. The HLS `<video>` element uses browser-native controls and is not marked `autoplay`; browsers may require a user gesture before playback. The WebRTC player requests `autoStart` with audio enabled, but browser autoplay policy can still require user interaction. A blocked autoplay attempt is therefore a browser policy outcome, not a supported guarantee that BitRiver bypasses.

### Fullscreen and picture-in-picture

HLS playback uses the browser's native video controls. Fullscreen and picture-in-picture are available only when the browser/device exposes them through those controls. BitRiver Live does not currently ship a separate cross-browser fullscreen/PiP control layer and does not make a stronger support promise than the underlying browser.

### Quality selection

The HLS path uses `hls.js` automatic adaptive bitrate behavior and records rendition changes for QoE telemetry. A manual cross-browser rendition selector is not part of the v1.2.3 support claim. The viewer may display rendition/debug information, but operators should not advertise manual quality selection as uniformly supported until a dedicated qualified control is shipped.

## Mobile behavior

The release-blocking mobile projects require touch activation of primary navigation/auth surfaces and assert that the document does not horizontally overflow at the emulated supported viewport. Existing Chromium regression tests additionally cover multiple narrow widths, sidebar focus management, Escape/backdrop behavior, and populated channel/chat layouts.

Soft-keyboard behavior is browser/OS dependent and cannot be fully represented by desktop-hosted Playwright emulation. The mobile compatibility gate verifies that the composer/auth/navigation surfaces remain reachable and layout-safe; physical-device qualification is still required before claiming behavior tied specifically to Android or iOS virtual keyboards, orientation sensors, or browser chrome.

## Browser-version policy

- CI qualifies the browser engines bundled with the repository-pinned Playwright version.
- Dependency updates that change Playwright/browser revisions must pass the complete compatibility matrix before merge.
- A regression isolated to one engine is release-blocking for that supported class. The project does not use retries to hide it.
- If an upstream browser defect cannot be fixed safely, the support boundary must be explicitly narrowed here with the upstream issue, affected versions, user-visible fallback, owner, and removal/review date.
- Real Safari/iOS/Android version claims require evidence from those shipping browsers/devices; Playwright engine/device emulation is necessary evidence, not sufficient evidence for hardware-specific certification.

## Unsupported or not-yet-qualified combinations

The following are not implied by the supported matrix:

- every historical browser version;
- embedded webviews, smart-TV browsers, game-console browsers, or unusual Chromium forks;
- physical Safari/iPhone/Android certification from Playwright emulation alone;
- browser-wide WebRTC success across arbitrary NAT/TURN/firewall environments;
- guaranteed audible autoplay;
- uniform fullscreen/PiP behavior beyond native browser controls;
- uniform manual rendition selection.

Unsupported playback must fail visibly through the viewer's loading/reconnecting/unavailable states rather than silently reporting healthy playback.

## Release evidence

For a stable candidate, retain or link:

1. the protected Viewer CI run containing all five compatibility projects plus the desktop Chromium regression;
2. the Playwright HTML report for browser-specific failures/triage when needed;
3. the full-stack real-media golden-path result for the exact release candidate;
4. any physical-browser/device results used to make a claim beyond the Playwright engine matrix;
5. documented exclusions or temporary support-boundary reductions.

This matrix is an executable support contract, not a marketing list. If CI, real-media evidence, and documentation disagree, the narrower proven boundary wins.
