# Repository instructions

This is a personal macOS bootstrap and shell-config repo deployed to `~/.melvin/config`. Read this file before making non-trivial changes.

## Keep this file fresh

When you make a change that matches a row below, update the named section in the same change. Out-of-date instructions are worse than missing ones — future agents will trust this file, so don't let it lie. If your change doesn't fit any row but future-you would still want to know about it, add a row.

| If you change… | Update this section |
|---|---|
| Top-level layout (new/removed dir or top-level dotfile) | [Layout](#layout) |
| `install.sh` setup phase, subsystem, or env-var contract | [Bootstrap flow](#bootstrap-flow) + [Env vars](#env-vars) |
| Claude config sync semantics (base SHA, decisions cache, merge strategy, sync-state files) | [Claude Code config sync](#claude-code-config-sync) |
| `tests/*.sh` added/removed/renamed, or `.github/workflows/*.yml` jobs | [Tests & CI](#tests--ci) |
| Top-level helper in `config.zshrc` (the `run`/`add`/`install`/`lint`/`wt`/`cl` family) | [Shell config](#shell-config) |
| Symlinked dotfile set in `copy_config_files` (array at install.sh:365-370) | [Layout](#layout) + [Bootstrap flow](#bootstrap-flow) |

## What this repo is

A personal dotfiles + macOS bootstrap repository. The source of truth is this repo at `~/.melvin/config`; `$HOME` gets thin symlinks back into it, plus a small set of materialized config files (`~/.zshrc`, `~/.gitconfig`, `~/.zprofile`) generated on first install.

There's no application code and no compiled artifacts. The "build" is `bash install.sh`.

## Layout

| Path | What it is |
|---|---|
| `install.sh` | Bootstrap entrypoint. Idempotent — handles first install and updates. |
| `base.zshrc` | Stable shell entrypoint sourced from generated `~/.zshrc`. Oh My Zsh + theme + the periodic update prompt. |
| `config.zshrc` | Day-to-day aliases and helper functions. Sourced from `base.zshrc`. |
| `.gitconfig` / `.gitmessage` | Canonical git config. `~/.gitconfig` is materialized to `[include]` this file. |
| `.golangci.yml` / `lsd.yaml` | Tool configs. `.golangci.yml` is symlinked into `$HOME`; `lsd.yaml` is passed to `lsd` via the `lsd` alias in config.zshrc. |
| `.oh-my-zsh/` | Tracked OMZ payload, symlinked to `~/.oh-my-zsh`. |
| `.emacs.d/` | Tracked emacs config, symlinked to `~/.emacs.d`. |
| `.bin-remote/` | Scripts, symlinked to `~/.bin-remote`. |
| `.claude/` | Curated Claude Code config: `settings.json` + `skills/` + `agents/` + `commands/`. Synced into `~/.claude/` per the [Claude Code config sync](#claude-code-config-sync) flow. Runtime subdirs (`projects/`, `sessions/`, `plugins/`, …) are gitignored. |
| `.github/workflows/` | CI definitions. |
| `.github/skills/` | Skills used by GitHub-side agent automation. |
| `skills/` | Local skills consumed by other Claude/agent surfaces. |
| `tests/` | Bash test scripts run by CI and locally. |
| `.ai/plans/` | Persisted plan markdown (planning agents). |
| `.vscode/` | Editor settings. |
| `README.md` | Just the curl-bootstrap one-liner. |

## Bootstrap flow

`install.sh` runs `main` (install.sh:1214) in this order:

1. `setup_personal_computer_flag` / `setup_skip_existing_flag` — capture the two interactive flags.
2. **If `~/.melvin/config` already exists:** `update_config_repo` does a `git pull`. If new commits were fetched, the script `exec`s into the freshly-pulled `install.sh`, propagating `PERSONAL_COMPUTER`, `SKIP_CONFIG_FILE_SETUP`, and `SKIP_CLAUDE_MERGE_PROMPTS` (install.sh:1222-1226). The original process is replaced — brew install does **not** run twice.
3. `install_homebrew` → `upgrade_homebrew_packages` → `install_packages`. Formulae + common casks always; `personal_casks` array gated on `PERSONAL_COMPUTER`.
4. `setup_ssh` — creates `~/.ssh/default` ed25519 keypair if missing.
5. `gh auth status` → populates `GH_STATUS` / `GH_ACCOUNT` for downstream branches.
6. `clone_config_repo` — only if `$CONFIG_DIR` does not exist.
7. `copy_config_files` — symlinks `.oh-my-zsh`, `.emacs.d`, `.bin-remote`, `.golangci.yml` into `$HOME`, then calls `claude_setup`.
8. `setup_zshrc` / `setup_gitconfig` / `setup_gpg` — first-install only; each guards on the destination existing.
9. `setup_github` — `gh auth login -w` if not already authenticated.
10. Stamp `.last_update_check`, `print_remaining_tasks`, `reload_zshrc`.

### Cask install resilience

`install_cask` (install.sh:149) treats casks whose app is currently running as "skip, don't fail", and collects per-cask failures into `SKIPPED_CASK_UPDATES` / `FAILED_CASK_UPDATES` arrays. `print_skipped_cask_updates` and `print_failed_cask_updates` summarize at the end. Don't replace this with `set -e` failures — the install is intentionally allowed to limp and report.

### Claude Code config sync

`claude_setup` (install.sh:1189) syncs the four curated items (`settings.json`, `skills/`, `agents/`, `commands/`) from the repo's `.claude/` into `~/.claude/`. Two modes:

- **Personal computer (`PERSONAL_COMPUTER=true`)**: `claude_install_symlink` makes `~/.claude/<item>` a symlink to the repo copy. `~/.claude/` itself is a real directory because Claude Code writes runtime state there — only the four curated items become symlinks.
- **Otherwise**: `claude_install_or_merge_copy` copies + 3-way merges. Base = repo content at the SHA in `.claude/.sync-state/last-sync-commit`. Local = `~/.claude/<file>`. Remote = current repo HEAD. Only true conflicts (both sides diverged differently from base) prompt the user.

Three persistent state files live in `.claude/.sync-state/` (gitignored):

- `last-sync-commit` — anchor SHA for the merge base. Only advanced after a fully-clean sync (no skipped conflicts); see `claude_advance_sync_commit` (install.sh:731).
- `decisions.json` — remembered "always keep local" / "always take remote" choices per conflict path.
- `README.md` — explains the directory to humans who poke at it.

Non-interactive override: `SKIP_CLAUDE_MERGE_PROMPTS=keep-local|take-remote|skip` (install.sh:662). The `skip` value sets `CLAUDE_HAD_SKIPS=1`, which holds `last-sync-commit` in place so unresolved conflicts re-surface on the next run.

### Env vars

| Var | Read by | Effect |
|---|---|---|
| `PERSONAL_COMPUTER` | install.sh | Gates personal casks, gpg signing, default git org, and Claude sync mode. Persisted via `~/.zprofile`. |
| `SKIP_CONFIG_FILE_SETUP` | install.sh | Skip-don't-prompt on dotfile collisions. From interactive prompt or set explicitly. |
| `SKIP_CLAUDE_MERGE_PROMPTS` | install.sh (Claude sync) | `keep-local` / `take-remote` / `skip` — bypass merge prompts. Initialized to empty at the top of install.sh so `set -u` test harnesses don't trip on the re-exec branch. |
| `GH_STATUS` / `GH_ACCOUNT` | install.sh | Parsed from `gh auth status --json`. Gates auto-running `omz update` on the owner account, plus `setup_github`. |
| `DEV_ROOT` / `WORKTREES_ROOT` / `REPOS_ROOT` / `SDKS_ROOT` | shell + `wt` helpers | Written to `~/.zshrc` during `setup_zshrc`. |
| `GIT_HOST` / `GIT_CLONE_USER_NAME` | shell helpers (`cl`, `wt`) | Written to `~/.zshrc` during `setup_zshrc`. |

When you read a new env var inside `main` *before* the re-exec branch (install.sh:1222), add it to the `exec env` propagation list — otherwise the post-pull process will lose it.

## Shell config

- `base.zshrc` — Oh My Zsh setup, theme `nivl`, plugin list, `PATH` baseline, `zsh-syntax-highlighting`, and `_melvin_check_update` (a 14-day prompt that re-runs `install.sh`). Sources `config.zshrc` at the end.
- `config.zshrc` — marker-based dispatch helpers + utility functions. When adding a new "this means different things in different repo types" helper, follow the existing style (test for marker files, then dispatch):
  - **Project dispatchers**: `run`, `add`, `install`, `lint` — pick behavior from markers (`yarn.lock`, `pnpm-lock.yaml`, `go.mod`, `manage.py`, …).
  - **`cl`** — clone any repo into `$REPOS_ROOT/<host>/<org>/<repo>` regardless of where you are.
  - **Worktree family**: `wt <branch>` creates `$WORKTREES_ROOT/<repo>/<branch>` and copies a curated set of gitignored files (env files, SDK caches, …) into it; `wt_done` cleans up; `wt_exit` returns to the main checkout. Internal helpers: `_should_copy_wt_ignored_path`, `_confirm_wt_done_delete`, `_cleanup_wt_worktree_dirs`.
  - **Other utilities**: `rec`, `findport`, `erase`, `code`, `is-go-repo`, `keygen`.

## Tests & CI

CI workflow at `.github/workflows/tests.yml` runs on `pull_request` and pushes to `main`. All jobs run on `macos-latest`:

| Job | Command |
|---|---|
| `shell-syntax` | `bash -n install.sh && zsh -n base.zshrc config.zshrc` |
| `install-regression` | `bash tests/install_cask_update_test.sh` |
| `worktrees` | `bash tests/wt_ignored_copy_test.sh` |
| `claude-sync` | `bash tests/claude_sync_test.sh` |

Test scripts use `set -euo pipefail`, so unbound-variable bugs in `install.sh` surface here even though `install.sh` itself only sets `-e`. The install regression test sources `install.sh` minus its trailing `main` call, stubs side-effecting functions, and exercises four scenarios: fresh install, update with no new commits, re-exec after pull, and `SKIP_CONFIG_FILE_SETUP` preset. Run any of them locally with `bash tests/<script>.sh`.

## Conventions

- **Idempotent**: `install.sh` must handle first install and repeated updates. Every destination check (`if [ -e "$ZSHRC" ]; then return; fi`) exists for a reason.
- **Interactive but scriptable**: prompts loop until valid input; non-interactive overrides exist for the gating decisions (`PERSONAL_COMPUTER`, `SKIP_CONFIG_FILE_SETUP`, `SKIP_CLAUDE_MERGE_PROMPTS`). New gates should follow the same shape: env-var fast-path, prompt as fallback.
- **Symlink, don't copy**: shared assets stay as the source of truth in this repo. The materialized exceptions are files that need machine-specific values (`~/.zshrc`, `~/.gitconfig`, `~/.zprofile`, `~/.gnupg/gpg-agent.conf`).
- **Layered shell**: machine setup in `install.sh`, shell startup in `base.zshrc`, day-to-day helpers in `config.zshrc`. Don't blur the layers.
- **`set -e` only in install.sh**: the script deliberately uses `set -e` (not `-u`). CI adds `-u` on top via the test harness. If you reference an env var that may be unset, default it (`${VAR:-}`) or initialize it at the top of `install.sh`.
- **`.oh-my-zsh/` and `.emacs.d/`**: tracked payloads. Targeted edits are fine; broad refactors there are risky and rarely worth it.
