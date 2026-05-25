# Personal bootstrap and configuration for macOS

Personal dotfiles + macOS bootstrap. Installs Homebrew packages, materializes `~/.zshrc` / `~/.gitconfig` / `~/.gnupg/gpg-agent.conf`, symlinks config files like `~/.emacs.d` into the repo, syncs `~/.claude/` config, and generates an SSH keypair if absent.

## Prerequisites

- **macOS** (Apple Silicon or Intel — the installer handles both).
- **`curl`** (preinstalled on macOS).
- **An SSH key registered with GitHub** that can pull `git@github.com:Nivl/Config.git`. The installer clones over SSH.

Homebrew, `git`, and `go` are installed by the bootstrap if they're not already present.

## Install

```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Nivl/Config/refs/heads/main/install.bootstrap.sh)"
```

Preview what setup would do without writing anything:

```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Nivl/Config/refs/heads/main/install.bootstrap.sh)" -- --dry-run
```

Dry-run prints a per-file unified diff of every change setup *would* make to your home directory, then exits. The bootstrap's own prerequisite steps (Homebrew install, Go install, repo clone, binary build) still run for real — only the user-facing config phase is previewed.

## Commands

After the initial install, `melvin-config` is on your `PATH` (via `shared_config/.bin-remote/`). One binary, all commands:

| Command | What it does |
|---|---|
| `melvin-config setup` | Main install/reconcile loop — packages, claude sync, dotfiles, configgen, ssh, github. |
| `melvin-config update [args…]` | Pulls the repo, rebuilds the binary, and re-runs `setup` with whatever args you pass. Replaces the old `update-cfg` shell function. |
| `melvin-config check-update` | Shell-startup hook (called from `base.zshrc`). Silent no-op unless 14+ days have passed, then prompts Y/N. |
| `melvin-config claude sync` | Re-runs only the Claude Code config sync (symlink or copy/merge mode). |
| `melvin-config claude perms {allow\|ask\|deny} {add\|remove}` | Mutate `permissions.{allow,ask,deny}` in `shared_config/.claude/settings.json` and commit. See [Managing Claude permissions](#managing-claude-permissions). |
| `melvin-config install packages` | Re-runs only the Homebrew install loop. |
| `melvin-config version` | Print version info — the build SHA when invoked via the bootstrap, or `dev` for ad-hoc local builds. |
| `melvin-config help` | Print the full help. |

Every command accepts `--help`. `setup`, `update`, and `claude perms` accept `--dry-run` (or `MELVIN_DRY_RUN=true`) to preview without writing.

## Update

```bash
melvin-config update            # pull + rebuild + run setup
melvin-config update --dry-run  # preview, no writes, no commits
```

You can pass any `setup` flag through, e.g. `melvin-config update --personal --merge-resolution=keep-local`.

You can also re-run the curl-bash install command — it does a `git pull` instead of a clone when `~/.melvin/config` already exists, then runs setup the same way.

## Managing Claude permissions

`melvin-config claude perms` mutates either `shared_config/.claude/settings.json` or `shared_config/.claude/hooks/git-safe-subcommands.py` (or both) and commits the change. Routing depends on the rule kind:

- **Non-git `--bash`, `--read`, `--fetch`, `--skill`** land in `settings.json`. Each `--bash` value also gets its PATH-resolved twin (e.g. `Bash(ls *)` adds `Bash(/bin/ls *)` too) so the rule matches whichever form Claude Code emits.
- **Git `--bash`** (any value whose first token is `git` or an absolute path ending in `/git`) lands in the Python hook file as a prefix tuple in `ALLOW_PREFIXES` / `ASK_PREFIXES` / `DENY_PREFIXES`. The hook then evaluates every `git …` command against those tables (deny > ask > allow) and emits a `permissionDecision` accordingly — independent of `-C <path>` or which absolute git binary was invoked.

Git rules **must** end with a trailing `*` (e.g. `git show *`, not `git show`). The hook always allows trailing args after a matched prefix, so exact-match rules don't fit the shape — for those, hand-edit `git-safe-subcommands.py`.

Add multiple rules in one invocation — `--bash` / `--read` / `--fetch` / `--skill` are each repeatable AND comma-separable. A mixed batch stages both files in one commit:

```bash
melvin-config claude perms allow add \
  --bash 'git status *' \
  --bash 'ls *,pwd' \
  --read 'shared/notes.md' \
  --fetch 'npm.io'
```

Preview without writing or committing:

```bash
melvin-config claude perms allow add --bash 'git status *' --dry-run
```

Remove rules — same flag shape, opposite direction:

```bash
melvin-config claude perms allow remove --bash 'git status *'
```

Re-categorize an existing rule (e.g. demote from `allow` to `ask`). Without `--force`, the cross-list conflict errors so you know what's about to move:

```bash
melvin-config claude perms ask add --bash 'rm *' --force
```

`melvin-config claude perms` only runs on a personal computer (`PERSONAL_COMPUTER=true`).

## Environment variables

Every variable below also has a matching `--flag` on `melvin-config setup`. Flag wins over env when both are set; the env-only form is convenient for `~/.zprofile` or one-off sessions.

| Variable | What it does |
|---|---|
| `PERSONAL_COMPUTER` | `true` enables personal apps (proton-*, daisydisk, …), GPG-signed commits, and the symlink mode for Claude Code config sync. Prompted on first install and persisted to `~/.zprofile`. |
| `DEV_ROOT` | Root directory for your dev workspaces. Defaults to `$HOME/dev`. Drives `WORKTREES_ROOT`, `REPOS_ROOT`, `SDKS_ROOT` (all derived as `$DEV_ROOT/<name>`). |
| `GIT_HOST` | Your SSH git host, e.g. `git@github.com`. Used by the `wt` / `cl` worktree helpers. Prompted on first install if unset. |
| `GIT_CLONE_USER_NAME` | Default org or user used when cloning short-form `owner/repo` paths. On personal computers this defaults to `Nivl`. |
| `CLAUDE_MERGE_RESOLUTION` | Pre-resolve Claude config merge conflicts non-interactively. One of `keep-local`, `take-remote`, `skip`. Useful for CI / unattended runs. |
| `HOMEBREW_PREFIX` | Override the auto-detected Homebrew prefix used for the `pinentry-mac` path in `~/.gnupg/gpg-agent.conf`. Falls back to `$(brew --prefix)`. |
| `MELVIN_DRY_RUN` | `true` enables preview mode. Identical to passing `--dry-run`. |

`WORKTREES_ROOT`, `REPOS_ROOT`, and `SDKS_ROOT` are written into `~/.zshrc` at install time as `$DEV_ROOT/worktrees`, `$DEV_ROOT/repos`, `$DEV_ROOT/sdks`. Override `DEV_ROOT` to move them; they're consumed by the `wt` / `cl` helpers in `config.zshrc`.
