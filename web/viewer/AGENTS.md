# `web/viewer` Guidance

Next.js 13 viewer (App Router). Read `web/AGENTS.md` and the root `AGENTS.md` first.

## Architecture
- Uses the App Router with server components under `app/`. Shared API calls live in `lib/viewer-api.ts`; update it when backend payloads change.
- Stateful pages such as `app/page.tsx` (directory) and channel routes must fetch data server-side, handle loading/error states, and surface empty states that match the control centre.
- Global state/providers (theme, auth) live under `components/` or `providers/`—reuse them when adding features.

## Testing
- Unit tests live under `__tests__/`. Integration + accessibility tests reside in `tests/` (Playwright). When adjusting UX flows (channel pages, chat panel, auth), add or update Playwright specs such as `tests/channel.spec.ts`.
- Lint/test commands: `npm run lint`, `npm run test`, `npm run test:playwright`. Ensure Playwright dependencies are installed (`npx playwright install --with-deps` for CI parity).

## Manual QA
- For visual/escaping-sensitive changes, run the checklist in `web/manual-qa.md` and attach screenshots when relevant.

## Before opening a PR
- Keep API fetches typed (update `types.ts` or the relevant schemas).
- Sync docs or in-app help text with README/docs when user journeys change.
- Run the lint/unit/Playwright commands above and capture failing output in the PR if needed.
