## Scope (current change)
- Align the public viewer shell and homepage with the established BitRiver ecosystem visual language shown in the supplied Radio and Visual Archive references.
- Replace generic rounded SaaS styling with a compact broadcast-console system: near-black surfaces, gold structure, cyan live state, red warnings, square geometry, and restrained monospace metadata.
- Preserve the existing route, auth, search, theme, loading, error, and empty-state behavior.
- Make a fresh installation feel operational and intentional even before its first channel goes live.
- Keep creator/admin product zones and the deployment contract untouched.

## Assumptions
- The ecosystem screenshots are visual direction, not a request to clone product-specific Radio or archive content.
- The public viewer should be calmer and more legible than the Radio console while sharing its visual DNA.
- Existing light-theme support remains part of the viewer contract and needs an adapted high-contrast treatment.
- Existing semantic markup and navigation policy should remain stable unless a small copy change materially improves the empty-install experience.

## Risks
- Broad token changes can regress channel, chat, or creator screens; scope overrides to shared primitives and homepage/navigation selectors, then run focused and full viewer checks.
- Dense console styling can hurt mobile usability; verify desktop and mobile screenshots, overflow, focus states, and reduced-motion behavior.
- Decorative scan lines and glow can reduce readability; keep effects low contrast and never place them above interactive content.

## Test plan
- `npm.cmd --prefix web/viewer run test -- directoryPage navbar viewerShell`
- `npm.cmd --prefix web/viewer run lint`
- `npm.cmd --prefix web/viewer run build`
- Browser screenshots and DOM checks at desktop and mobile widths.
- `git diff --check`
- `powershell -ExecutionPolicy Bypass -File .\scripts\verify.ps1 --viewer` when available; otherwise record the host prerequisite blocker.
