# Repository instructions

## Build, test, and lint commands

This repository does not define a formal root-level build, test, or lint suite such as `make`, `task`, `npm`, or CI scripts.

Use these commands when validating changes:

- Bootstrap/install flow: `bash install.sh`
- Repo entrypoint from the README: `/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Nivl/Config/refs/heads/main/install.sh)"`
- Bash syntax check: `bash -n install.sh`
- Zsh syntax check: `zsh -n base.zshrc config.zshrc`

There is no single-test command because the repo does not include an automated test suite.

## High-level architecture

This is a personal macOS bootstrap and shell-config repository, not an application or library.

- `install.sh` is the main bootstrap entrypoint. It installs Homebrew packages, prepares SSH and GPG, checks GitHub auth with `gh`, clones or updates this repo into `~/.melvin/config`, symlinks shared assets into `$HOME`, and creates first-run `~/.zshrc` and `~/.gitconfig` files when missing.
- `base.zshrc` is the stable shell entrypoint that a generated `~/.zshrc` sources. It configures Oh My Zsh, theme/plugins, baseline `PATH`, syntax highlighting, and the periodic self-update prompt for this config repo.
- `config.zshrc` is the user workflow layer loaded at the end of `base.zshrc`. It contains PATH additions, aliases, and helper functions such as `run`, `add`, `install`, and `lint` that dispatch based on markers in the current project (`yarn.lock`, `pnpm-lock.yaml`, `go.mod`, `manage.py`, etc.).
- Root dotfiles such as `.gitconfig`, `.gitmessage`, `.golangci.yml`, and `lsd.yaml` act as canonical shared config that gets linked or sourced from the installed machine.
- `.oh-my-zsh/` and `.emacs.d/` are tracked config payloads that are deployed by `install.sh`. Treat them as managed assets; most repo maintenance happens in the root scripts and config entrypoints unless a task explicitly targets those directories.

## Key conventions

- Preserve the split between bootstrap logic and interactive shell behavior: machine/setup flow belongs in `install.sh`, shell startup belongs in `base.zshrc`, and day-to-day aliases/functions belong in `config.zshrc`.
- Keep installer changes idempotent. `install.sh` is written to handle both first-time setup and updates, including prompting around existing files and updating the repo in place.
- Keep the installer interactive and fail-fast. Prompt helpers loop until valid input, and `set -e` is intentional; do not replace these flows with silent fallbacks.
- Prefer extending the existing environment flags and decision points instead of hardcoding behavior. Important branches already depend on `PERSONAL_COMPUTER`, `SKIP_CONFIG_FILE_SETUP`, `GH_STATUS`, and `GH_ACCOUNT`.
- When adding repo-type helpers in `config.zshrc`, follow the existing marker-based dispatch style used by `run`, `add`, `install`, and `lint`.
- Preserve the symlink-based deployment model. Shared assets in this repo are meant to remain the source of truth rather than being copied and edited independently in `$HOME`.
- Be deliberate when editing `.oh-my-zsh/` or `.emacs.d/`. They are part of the installed payload, but broad cleanup or refactors there are riskier than changes to the root bootstrap/config files.
