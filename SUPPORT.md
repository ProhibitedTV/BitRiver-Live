# Support

BitRiver Live is maintained in the open, but it does not offer managed hosting or a support SLA.

The best-supported path in this repository is the documented single-host Docker Compose deployment built around `deploy/docker-compose.yml` and the repository-root `.env`.

The current public candidate is
[`v1.2.3-rc.12`](https://github.com/ProhibitedTV/BitRiver-Live/releases/tag/v1.2.3-rc.12).
Include that exact tag (or your commit SHA) in package and runtime reports.

## Start here first

- Installation and first success: [`README.md`](README.md) and [`docs/quickstart.md`](docs/quickstart.md)
- Ubuntu/XOA package installation: [`docs/installing-on-ubuntu.md`](docs/installing-on-ubuntu.md)
- Supported production baseline: [`docs/production-single-host.md`](docs/production-single-host.md)
- Security hardening: [`docs/security.md`](docs/security.md)
- Release and upgrade operations: [`docs/production-release.md`](docs/production-release.md) and [`docs/upgrades.md`](docs/upgrades.md)

## Before opening an issue

Please gather the smallest useful set of details:

- OS and shell
- Docker and Compose versions
- whether you used a source checkout, packaged launcher, systemd install, or another documented path
- image tag or commit SHA
- the exact command you ran
- redacted output from the failing step

Useful checks:

```bash
bash deploy/check-env.sh
go run ./cmd/bitriver smoke --env-file ./.env
docker compose --env-file .env -f deploy/docker-compose.yml ps
```

For an installed Ubuntu package, prefer the host-manager surface:

```bash
sudo bitriver-host doctor
sudo bitriver-host status
sudo bitriver-host logs
```

If you are contributing code, run the recommended repo gate when practical:

```bash
./scripts/verify.sh
```

## Where to ask for what

- Security vulnerability: follow [`SECURITY.md`](SECURITY.md) and do not open a public issue with exploit details.
- Reproducible bug, docs problem, or packaging regression: open a GitHub issue with the steps, environment, and redacted output.
- Small fix ready to contribute: open a pull request and include the commands you ran.

## What maintainers can help with most effectively

- The supported single-host Compose path
- Onboarding and documentation gaps
- Release and packaging regressions
- Focused bugs with a clear reproduction path

## What may be out of scope for repo support

- Custom Kubernetes or multi-host topologies
- Bespoke reverse-proxy, CDN, or networking setups beyond the documented guides
- Capacity planning or incident response for private production deployments
- Managed-service expectations
