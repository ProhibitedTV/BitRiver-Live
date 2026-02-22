# PLAN

## Scope (current change)
- Add a short required self-check footer section to root `AGENTS.md`.
- Ensure the footer explicitly points agents to `./scripts/verify.sh` as the default validation command.
- Keep wording concise and checklist-oriented so agents can append it before finishing runs.

## Assumptions
- This is a documentation-only workflow change; no runtime behavior or deployment contract files are affected.
- Existing `AGENTS.md` structure remains intact, with the new footer added near end-of-file guidance sections.

## Implementation approach
1. Add a compact “finish-run self-check” checklist to `AGENTS.md`.
2. Include all required prompts from the request, with a command-list item anchored on `./scripts/verify.sh`.
3. Keep text brief to minimize instruction noise.

## Technical plan
1. Read `SPEC.md` and relevant repo docs/code in read-only mode.
2. Update this file with scope, assumptions, and implementation approach.
3. Break implementation into ordered tasks in `TASKS.md`.
4. Execute tasks top-to-bottom; do not skip ahead.

## Risks
- Checklist could be too verbose and reduce compliance.
- Footer placement could conflict with existing agent guidance if not clearly marked required.
- Plan drift if `TASKS.md` status updates are missed during doc edits.

## Test plan
- Run a documentation sanity check by inspecting the updated `AGENTS.md` section content.
- Confirm required checklist bullets are present, concise, and include `./scripts/verify.sh`.
- Record checks and outcomes in `TASKS.md` immediately after edits.
