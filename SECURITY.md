# Security Policy

Security reports are welcome and appreciated.

## Reporting a vulnerability

Please do not open a public GitHub issue with exploit details.

Use one of these private paths instead:

1. GitHub's private vulnerability reporting or security advisory flow for this repository, if it is enabled.
2. A direct private contact path for the repository owner on the hosting platform.

When you report an issue, include:

- affected version, branch, or image tag
- deployment shape used (`deploy/docker-compose.yml`, launcher install, and so on)
- reproduction steps
- impact and any known mitigations

## What to expect

- We will acknowledge receipt as soon as practical.
- We will try to confirm severity and reproduction details before discussing timelines.
- We prefer coordinated disclosure after a fix or mitigation is available.

## Scope

Please report security issues such as:

- authentication or authorization bypass
- credential or secret exposure
- unsafe default configuration
- remote code execution or container breakout risk
- injection flaws
- SSRF, CSRF, or privilege-escalation issues

Operational hardening guidance lives in [`docs/security.md`](docs/security.md) and [`docs/secrets-hardening.md`](docs/secrets-hardening.md).

## Repository change controls

The `main` branch requires a pull request, an up-to-date successful `Merge
gate`, and resolved review conversations. The rule applies to administrators;
force pushes and branch deletion are disabled. Conditional child checks may be
skipped only when the aggregate gate confirms their path selectors did not
require them.

Break glass is reserved for a repository-wide incident where the normal pull
request path cannot operate. A maintainer must record the reason and exact
protection change in an issue or security advisory, make the smallest possible
change, restore protection immediately, and follow with a normal audited pull
request and CI evidence. A failing test or urgent release is not by itself a
reason to bypass the gate.

## Supported versions

Until the first public stable release is tagged, security fixes should be assumed to land on `main`.

After the first stable release, the project will support:

- the latest stable release tag
- `main` for unreleased fixes
