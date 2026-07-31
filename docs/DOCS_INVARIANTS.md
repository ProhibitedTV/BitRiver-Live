# Documentation Invariants

Run these checks before merging documentation or deployment-contract changes:

- `./scripts/generate-contract-doc.sh --check`
- `./scripts/check-contract-invariants.sh`
- `./scripts/check-doc-installer-language.sh`
- `python3 -m unittest scripts/check_doc_links_test.py`
- `python3 scripts/check_doc_links.py`

The contract invariants check validates the deployment files and generated-
contract assumptions documented in `docs/contract.md`. The installer-language
check keeps the public README, quickstart, package guide, status, and release
notes aligned with the current published candidate and real product captures.
