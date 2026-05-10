#!/bin/zsh
#
# install.bootstrap.sh — production entrypoint for melvin-config.
#
# Order of operations:
#   1. Install brew if missing (chicken-and-egg; must be shell).
#   2. Install git + go via brew (minimum runtime to proceed).
#   3. Clone or update the config repo. Re-exec into the freshly-pulled
#      bootstrap if new commits arrived. The _MELVIN_REEXECED guard
#      prevents an infinite loop.
#   4. go build the melvin-config binary into the config repo's
#      shared_config/.bin-remote/ (added to PATH by base.zshrc; Go's
#      per-package cache makes incremental builds sub-100ms when
#      nothing changed).
#   5. exec into `melvin-config setup`. The Go binary owns the entire
#      runtime (prompts, package install, Claude sync, dotfile copy,
#      zshrc/gitconfig/gpg materialization, SSH key gen, GitHub auth,
#      reminders). install.sh was deleted in step 6 of the bash → Go
#      migration; the entire runtime is now Go.
#
# Env vars consumed:
#   PERSONAL_COMPUTER          - prompted at step 0 below if unset;
#                                propagated through re-exec; read by Go
#                                (userinput.Personal + Opts.Personal)
#   CLAUDE_MERGE_RESOLUTION    - propagated through re-exec; consumed by
#                                Go's internal/claude/sync/prompt to bypass
#                                merge prompts
#   MELVIN_DRY_RUN             - propagated through re-exec; consumed by
#                                Go's internal/cmd/setup.go to suppress
#                                file writes and side-effecting shellouts
#   _MELVIN_REEXECED           - internal; set by the re-exec branch

set -e

# Parse --dry-run from positional args. The flag is propagated to the
# Go binary via MELVIN_DRY_RUN. Bootstrap's prereq phases (brew, go,
# clone, build) still run for real — dry-run only scopes to the final
# `melvin-config setup` invocation.
for arg in "$@"; do
  if [ "$arg" = "--dry-run" ]; then
    export MELVIN_DRY_RUN=true
  fi
done

CONFIG_DIR="$HOME/.melvin/config"
# We build into shared_config/.bin-remote/ (under the config repo
# itself) rather than ~/.local/bin so the binary's location is owned
# by this repo and added to PATH by shared_config/base.zshrc. That
# in turn lets the shell-startup `melvin-config check-update` hook
# resolve the binary without depending on the user having
# ~/.local/bin already on their PATH.
BIN_DIR="$CONFIG_DIR/shared_config/.bin-remote"
BIN="$BIN_DIR/melvin-config"

# ---------- 0. PERSONAL_COMPUTER prompt (must precede the Go handoff) -------
#
# packages.Install reads Opts.Personal from os.Getenv("PERSONAL_COMPUTER")
# in cmd/melvin-config/setup.go. If the env var is unset when `melvin-config
# setup` runs, PersonalCasks (proton-*, daisydisk, …) are silently skipped.
# melvin-config setup's userinput.Personal prompt fires only when the env
# var is empty — pre-setting it here on the bootstrap side ensures the Go
# phase sees the value without re-prompting.

if [ -z "${PERSONAL_COMPUTER:-}" ]; then
  while true; do
    printf "Is this for a personal computer (y/n)? "
    # If stdin is closed (e.g. `curl … | zsh install.bootstrap.sh`),
    # read returns non-zero — abort rather than spin forever in the
    # "Invalid value" branch.
    read -r answer || { printf "\nNo input; aborting.\n" >&2; exit 1; }
    case ${answer:0:1} in
    "y" | "Y")
      export PERSONAL_COMPUTER=true
      break
      ;;
    "n" | "N")
      export PERSONAL_COMPUTER=false
      break
      ;;
    *) printf "Invalid value\n" ;;
    esac
  done
  # Append (not >) so we don't clobber content on re-runs — by the time
  # this fires a second time, ~/.zprofile typically has the brew shellenv
  # line from a prior run, plus any user-authored entries.
  # Match on the exact value (not just the key) so a user who switched
  # their answer between runs gets the new line appended below — zsh
  # sources top-to-bottom, so the later assignment wins. The duplicate
  # is intentional; rewriting in place risks mangling unrelated edits.
  # (Mirrors internal/userinput/personal.go's ensureZprofileLine.)
  if ! grep -qF "export PERSONAL_COMPUTER=$PERSONAL_COMPUTER" "$HOME/.zprofile" 2>/dev/null; then
    printf "\nexport PERSONAL_COMPUTER=%s\n" "$PERSONAL_COMPUTER" >>"$HOME/.zprofile"
  fi
fi

# ---------- 1. Homebrew (must be shell — chicken-and-egg) -------------------

if ! command -v brew >/dev/null 2>&1; then
  /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
  # Apple Silicon installs brew at /opt/homebrew/bin/brew; Intel installs
  # at /usr/local/bin/brew. Pick whichever the installer just wrote.
  if [ -x /opt/homebrew/bin/brew ]; then
    BREW_BIN=/opt/homebrew/bin/brew
  elif [ -x /usr/local/bin/brew ]; then
    BREW_BIN=/usr/local/bin/brew
  else
    echo "brew installation reported success but no brew binary found at /opt/homebrew/bin/brew or /usr/local/bin/brew" >&2
    exit 1
  fi
  echo "eval \"\$($BREW_BIN shellenv)\"" >>"$HOME/.zprofile"
  eval "$($BREW_BIN shellenv)"
fi

# ---------- 2. Minimum runtime: git + Go (must be shell, not Go) ------------

for pkg in git go; do
  command -v "$pkg" >/dev/null 2>&1 || brew install "$pkg"
done

# ---------- 3. Clone or update the config repo ------------------------------

if [ ! -d "$CONFIG_DIR" ]; then
  mkdir -p "$(dirname "$CONFIG_DIR")"
  git clone git@github.com:Nivl/Config.git "$CONFIG_DIR"
else
  if [ -z "${_MELVIN_REEXECED:-}" ]; then
    cd "$CONFIG_DIR"
    # Skip the pull on a dirty tree so set -e doesn't abort the
    # bootstrap with an opaque git error.
    if [ -n "$(git status --porcelain)" ]; then
      printf "Your config repository has uncommited changes, please commit or stash them before we can update\n"
      before=$(git rev-parse HEAD)
      after=$before
    else
      before=$(git rev-parse HEAD)
      git pull
      after=$(git rev-parse HEAD)
    fi
    if [ "$before" != "$after" ]; then
      exec env _MELVIN_REEXECED=1 \
        PERSONAL_COMPUTER="${PERSONAL_COMPUTER:-}" \
        CLAUDE_MERGE_RESOLUTION="${CLAUDE_MERGE_RESOLUTION:-}" \
        MELVIN_DRY_RUN="${MELVIN_DRY_RUN:-}" \
        zsh "$CONFIG_DIR/install.bootstrap.sh" "$@"
    fi
  fi
fi

# ---------- 4. Build the Go binary ------------------------------------------

mkdir -p "$BIN_DIR"
cd "$CONFIG_DIR"
# Stamp the binary with the config repo's current short SHA so
# `melvin-config version` returns something useful instead of "dev".
# CWD is $CONFIG_DIR per the `cd` above, so no `-C` needed.
VERSION="$(git rev-parse --short HEAD 2>/dev/null || echo dev)"
go build -ldflags "-X main.version=$VERSION" -o "$BIN" ./cmd/melvin-config

# ---------- 5. Hand off ------------------------------------------------------

exec "$BIN" setup "$@"
