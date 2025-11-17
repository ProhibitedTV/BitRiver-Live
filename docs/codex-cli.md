# Codex CLI guide

Use the Codex CLI to ask AI for edits without leaving your terminal. This guide mirrors the steps from the [Codex CLI docs](https://developers.openai.com/codex/cli) and applies them to this repository.

## Install the CLI

The CLI is published to PyPI. Install it with pip (or pipx) using the system Python so it lands on your shell `PATH`:

```bash
python3 -m pip install --user codex-cli
# or, if you prefer pipx
python3 -m pip install --user pipx
pipx ensurepath
pipx install codex-cli
```

Restart your shell if needed so the `codex` command is available. Verify with:

```bash
codex version
```

## Authenticate once

The CLI needs a Codex-capable OpenAI API key. Set it as an environment variable or let the helper prompt you:

```bash
# one-time setup
export OPENAI_API_KEY="sk-..."  # or set this in your shell profile
codex auth login                 # stores a short-lived token in ~/.config/codex
```

If you need to swap accounts or keys later, run `codex auth switch` or re-run `codex auth login`.

## Point the CLI at this repository

From the repository root (`BitRiver-Live`):

```bash
# initialize CLI metadata for this project (creates .codex/config.yaml)
codex init .

# start an interactive session scoped to the repo
codex edit .
```

The CLI reads the working tree to build context for the model. Run it from the root so it captures `docs/`, `cmd/`, `deploy/`, and `web/` together. Check the generated `.codex/config.yaml` into version control only if you want to share defaults; otherwise let it live locally.

## Common workflows

### Ask Codex to change files

```bash
# from the repo root
codex edit .
# describe the change when prompted (for example, "document how to run quickstart on macOS")
```

Codex proposes a diff. Review and accept the patch directly from the prompt, or save it for later with `--apply=false` and inspect the generated files first.

### Rerun Docker after edits

When you accept changes that affect the running containers (for example, `.env` defaults, Go code, or viewer assets), rebuild or restart the stack so Compose picks up the new state:

```bash
# still in the repo root
./scripts/quickstart.sh           # rebuilds images and restarts services
# or, if the stack is already running
export COMPOSE_FILE=deploy/docker-compose.yml
docker compose up -d              # reloads containers with the latest code
```

### Keep the CLI up to date

Upgrade periodically to pick up the latest prompts and workflow helpers:

```bash
python3 -m pip install --user --upgrade codex-cli
# or
pipx upgrade codex-cli
```

### Troubleshooting

- **`codex: command not found`** – Ensure `~/.local/bin` (for pip) or your pipx binary path is on `PATH`, then restart the shell.
- **Authentication failures** – Confirm `OPENAI_API_KEY` is set to a Codex-enabled key and rerun `codex auth login`.
- **Missing repo context** – Run the CLI from `/workspace/BitRiver-Live` (or wherever you cloned the repo) so it can read `.env`, `deploy/docker-compose.yml`, and the codebase.
- **Docker changes not applying** – Rerun `./scripts/quickstart.sh` or `docker compose up -d` after accepting Codex patches so the containers rebuild.
