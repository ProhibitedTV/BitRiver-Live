# Documentation Invariants

Run these checks before merging documentation or deployment-contract changes:

- `./scripts/generate-contract-doc.sh --check`
- `./scripts/check-contract-invariants.sh`

The contract invariants check validates the deployment files and generated-contract assumptions documented in `docs/contract.md`.
