# Contract Change Recipe (Env + Compose + Generated Configs)

Use this recipe whenever a PR changes environment contract behavior across:
- `deploy/.env.example`
- repo-root `.env` expectations
- `deploy/docker-compose.yml` `${VAR}` interpolation
- generated configs (`deploy/ome/Server.generated.xml`, SRS generated output if present)

## Minimal safe bundle (change these together)

For any contract variable you add, remove, or rename, update this minimum set in one PR:
1. `deploy/.env.example` (example/default value)
2. `deploy/docker-compose.yml` (all interpolation references)
3. The root `.env` contract documentation in `docs/contract.md`
4. Generated config artifacts affected by env rendering:
   - `deploy/ome/Server.generated.xml`
   - SRS generated config artifacts used by this repo (if touched by the variable)

If behavior or operator steps change, also update the user-facing docs that describe those steps.

## Required local commands (run from repo root)

Run these before requesting review:

```bash
./scripts/generate-contract-doc.sh
./scripts/verify.sh
./scripts/require-image-digests.sh
./scripts/test-quickstart.sh
```

Prerequisite note: `./scripts/verify.sh` requires `python3` on `PATH` because `./scripts/check-contract-invariants.sh` validates generated artifact references via an embedded Python block.

If your branch uses an equivalent smoke/quickstart verification flow, include that command output in the PR notes.

## Author checklist

- [ ] Variable names are consistent across `deploy/.env.example`, Compose interpolation, and generated configs.
- [ ] `docs/contract.md` reflects the new/changed root `.env` expectations.
- [ ] Generated files were re-rendered and committed when applicable (OME/SRS).
- [ ] `./scripts/generate-contract-doc.sh` completed successfully.
- [ ] `./scripts/verify.sh` completed successfully.
- [ ] `./scripts/require-image-digests.sh` completed successfully for production envs (`BITRIVER_LIVE_MODE=production` and `BITRIVER_DEPLOY_IMAGE_SOURCE=pull`).
- [ ] `./scripts/test-quickstart.sh` (or equivalent smoke verification) completed successfully.

## PR reviewer checklist

- [ ] PR contains the full minimal safe bundle (or explains why a listed file is unaffected).
- [ ] No dangling `${VAR}` references remain in `deploy/docker-compose.yml`.
- [ ] Contract docs and generated artifacts are synchronized with the env changes.
- [ ] Verification command results are attached and indicate success:
  - `./scripts/generate-contract-doc.sh`
  - `./scripts/verify.sh`
  - `./scripts/require-image-digests.sh` (production env contexts)
  - `./scripts/test-quickstart.sh` (or equivalent smoke verification)
