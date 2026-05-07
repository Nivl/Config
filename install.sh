#!/bin/zsh

set -e  # Exit on error

# =============================================================================
# Configuration
# =============================================================================

CONFIG_DIR="$HOME/.melvin/config"
ZSHRC="$HOME/.zshrc"
GITCFG="$HOME/.gitconfig"

# Optional env input: keep-local | take-remote | skip. See claude_resolve_conflict.
SKIP_CLAUDE_MERGE_PROMPTS="${SKIP_CLAUDE_MERGE_PROMPTS:-}"

# =============================================================================
# Helper Functions
# =============================================================================

# Prompt for yes/no input, returns 0 for yes, 1 for no
ask_yes_no() {
  local prompt="$1"
  while true; do
    printf "%s (y/n)? " "$prompt"
    read -r answer
    case ${answer:0:1} in
      "y"|"Y") return 0 ;;
      "n"|"N") return 1 ;;
      *) printf "Invalid value\n" ;;
    esac
  done
}

# Prompt for non-empty text input
ask_input() {
  local prompt="$1"
  local result=""
  while true; do
    printf "%s " "$prompt" >&2
    read -r result
    if [ -n "$result" ]; then
      echo "$result"
      return
    fi
  done
}

# =============================================================================
# Setup Functions
# =============================================================================

setup_personal_computer_flag() {
  if [ -n "$PERSONAL_COMPUTER" ]; then
    return
  fi

  PERSONAL_COMPUTER=false
  if ask_yes_no "Is this for a personal computer"; then
    PERSONAL_COMPUTER=true
  fi
  printf "\nexport PERSONAL_COMPUTER=%s" "$PERSONAL_COMPUTER" > "$HOME/.zprofile"
}

setup_skip_existing_flag() {
  if [ -n "$SKIP_CONFIG_FILE_SETUP" ]; then
    return
  fi

  SKIP_CONFIG_FILE_SETUP=false
  if ask_yes_no "Do you want to skip existing config files without prompting"; then
    SKIP_CONFIG_FILE_SETUP=true
  fi
}

# =============================================================================
# Installation Functions
# =============================================================================

install_homebrew() {
  if [ -n "$HOMEBREW_PREFIX" ]; then
    return
  fi

  /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
  (echo; echo 'eval "$(/opt/homebrew/bin/brew shellenv)"') >> "$HOME/.zprofile"
  eval "$(/opt/homebrew/bin/brew shellenv)"
}

upgrade_homebrew_packages() {
  brew upgrade
}

SKIPPED_CASK_UPDATES=()
FAILED_CASK_UPDATES=()
FAILED_CASK_FAILURE_REASONS=()

list_cask_apps() {
  brew info --cask --json=v2 "$1" | jq -r '
    .casks[0].artifacts[]? |
    select(type == "object" and has("app")) |
    .app[]?
  ' | sed -E 's#.*/##; s/\.app$//'
}

is_app_running() {
  local app_name="$1"

  if pgrep -ix "$app_name" > /dev/null 2>&1; then
    return 0
  fi

  if osascript -e "tell application \"$app_name\" to if it is running then return \"true\"" 2>/dev/null | grep -q '^true$'; then
    return 0
  fi

  return 1
}

is_cask_running() {
  local cask="$1"
  local app_name=""

  while IFS= read -r app_name; do
    if [ -z "$app_name" ]; then
      continue
    fi

    if is_app_running "$app_name"; then
      return 0
    fi
  done < <(list_cask_apps "$cask")

  return 1
}

extract_cask_failure_reason() {
  local stderr_file="$1"
  local reason=""

  reason="$(awk 'NF { line = $0 } END { print line }' "$stderr_file" | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//')"
  if [ -n "$reason" ]; then
    printf "%s\n" "$reason"
    return
  fi

  printf "Homebrew cask command failed without an error message\n"
}

install_cask() {
  local cask="$1"
  local brew_stderr_file=""
  local failure_reason=""

  if brew list --cask "$cask" > /dev/null 2>&1 && is_cask_running "$cask"; then
    printf "Skipping cask update for %s because its app is running\n" "$cask"
    SKIPPED_CASK_UPDATES+=("$cask")
    return
  fi

  brew_stderr_file="$(mktemp)"
  if brew install --cask "$cask" 2>"$brew_stderr_file"; then
    rm -f "$brew_stderr_file"
    return
  fi

  failure_reason="$(extract_cask_failure_reason "$brew_stderr_file")"
  rm -f "$brew_stderr_file"

  printf "Failed to install or upgrade cask %s: %s\n" "$cask" "$failure_reason"
  FAILED_CASK_UPDATES+=("$cask")
  FAILED_CASK_FAILURE_REASONS+=("$failure_reason")
}

install_casks() {
  local cask=""

  for cask in "$@"; do
    install_cask "$cask"
  done
}

print_skipped_cask_updates() {
  local cask=""

  if [ "${#SKIPPED_CASK_UPDATES[@]}" -eq 0 ]; then
    return
  fi

  printf "\nSkipped cask updates because the app is running:\n"
  for cask in "${SKIPPED_CASK_UPDATES[@]}"; do
    printf "\t- %s\n" "$cask"
  done
}

print_failed_cask_updates() {
  local i

  if [ "${#FAILED_CASK_UPDATES[@]}" -eq 0 ]; then
    return
  fi

  printf "\nFailed cask installs or upgrades:\n"
  for ((i = 1; i <= ${#FAILED_CASK_UPDATES[@]}; i++)); do
    printf "\t- %s: %s\n" "${FAILED_CASK_UPDATES[$i]}" "${FAILED_CASK_FAILURE_REASONS[$i]}"
  done
}

install_packages() {
  local common_casks=(
    zoom
    brave-browser
    warp
    docker
    raycast
    keka
    slack
    shottr
  )
  local beta_casks=(
    visual-studio-code@insiders
  )
  local personal_casks=(
    proton-drive
    proton-pass
    protonvpn
    daisydisk
    lulu
    ente
    yaak
    discord
    enpass
  )

  SKIPPED_CASK_UPDATES=()
  FAILED_CASK_UPDATES=()
  FAILED_CASK_FAILURE_REASONS=()

  # Core utilities
  brew install gnupg diff-so-fancy emacs pinentry-mac jq less grep zsh-syntax-highlighting shellcheck lsd gh 

  # Fonts
  brew install font-fira-code-nerd-font

  # Development tools
  brew install go golangci-lint go-task/tap/go-task nvm pnpm

  # AI
  brew install copilot-cli claude-code rtk

  # Common apps
  install_casks "${common_casks[@]}"

  # Beta apps
  install_casks "${beta_casks[@]}"

  # Personal apps
  if [ "$PERSONAL_COMPUTER" = true ]; then
    install_casks "${personal_casks[@]}"
  fi

  print_skipped_cask_updates
  print_failed_cask_updates
}

# =============================================================================
# SSH Functions
# =============================================================================

setup_ssh() {
  # Ensure .ssh directory exists with proper permissions
  if [ ! -d "$HOME/.ssh" ]; then
    mkdir -p "$HOME/.ssh"
    chmod 700 "$HOME/.ssh"
  fi

  if [ ! -e "$HOME/.ssh/default.pub" ]; then
    ssh-keygen -o -a 100 -t ed25519 -f "$HOME/.ssh/default"
  fi

  if [ ! -e "$HOME/.ssh/config" ]; then
    echo "IdentityFile $HOME/.ssh/default" > "$HOME/.ssh/config"
  fi
}

# =============================================================================
# Repository Functions
# =============================================================================

clone_config_repo() {
  mkdir -p "$CONFIG_DIR"
  cd "$CONFIG_DIR" || exit 1
  git clone git@github.com:Nivl/Config.git .
}

update_config_repo() {
  cd "$CONFIG_DIR" || exit 1

  if [ -n "$(git status --porcelain)" ]; then
    printf "Your config repository has uncommited changes, please commit or stash them before we can update\n"
    return 0
  fi

  local before
  before=$(git rev-parse HEAD)

  git pull

  # Update oh-my-zsh if authenticated as owner
  if [ "$GH_STATUS" == "success" ] && [ "$GH_ACCOUNT" == "Nivl" ]; then
    omz update
    if [ -n "$(git status --porcelain)" ]; then
      git add .oh-my-zsh
      git commit -m "update omz"
      git push
    fi
  fi

  local after
  after=$(git rev-parse HEAD)

  if [ "$before" != "$after" ]; then
    return 1
  fi
}

# =============================================================================
# Config File Functions
# =============================================================================

handle_existing_file() {
  local source="$1"
  local target="$2"

  while true; do
    printf "%s already exists\n" "$target"
    printf "\t1) Backup and Overwrite\n"
    printf "\t2) Overwrite\n"
    printf "\t3) Skip\n"
    read -r answer

    case ${answer:0:1} in
      "1")
        [ -e "$target.bpk" ] && rm -rf "$target.bpk"
        mv "$target" "$target.bpk"
        ln -s "$source" "$target"
        break
        ;;
      "2")
        rm -rf "$target"
        ln -s "$source" "$target"
        break
        ;;
      "3")
        printf "Skipping\n"
        break
        ;;
      *)
        printf "Invalid value\n"
        ;;
    esac
  done
}

copy_config_files() {
  local files=(
    ".oh-my-zsh"
    ".emacs.d"
    ".bin-remote"
    ".golangci.yml"
  )

  for file_name in "${files[@]}"; do
    local source="$CONFIG_DIR/$file_name"
    local target="$HOME/$file_name"

    if [ -e "$target" ]; then
      if [ "$SKIP_CONFIG_FILE_SETUP" = true ]; then
        printf "Skipping existing %s\n" "$target"
      else
        handle_existing_file "$source" "$target"
      fi
    else
      ln -s "$source" "$target"
    fi
  done

  mkdir -p "$HOME/.emacs-saves"

  claude_setup
}

# =============================================================================
# Zshrc Functions
# =============================================================================

ask_dev_root() {
  if [ -n "$DEV_ROOT" ]; then
    echo "$DEV_ROOT"
    return
  fi

  local default="$HOME/dev"
  local result=""
  printf "Where do you want your dev root to be? [%s]: " "$default" >&2
  read -r result
  if [ -z "$result" ]; then
    result="$default"
  fi
  echo "$result"
}

ask_git_org() {
  if [ "$PERSONAL_COMPUTER" = true ]; then
    echo "Nivl"
    return
  fi
  ask_input "What is the name of the git org?"
}

ask_git_host() {
  while true; do
    printf "Pick the git server\n" >&2
    printf "\t1) GitHub\n" >&2
    printf "\t2) Bitbucket\n" >&2
    printf "\t3) Gitlab\n" >&2
    printf "\t4) Custom\n" >&2
    read -r answer

    case ${answer:0:1} in
      "1") echo "git@github.com"; return ;;
      "2") echo "git@bitbucket.org"; return ;;
      "3") echo "git@gitlab.com"; return ;;
      "4")
        ask_input "Type the URL of the git server (usually in the form of git@github.com):"
        return
        ;;
      *) printf "Invalid value\n" >&2 ;;
    esac
  done
}

setup_zshrc() {
  if [ -e "$ZSHRC" ]; then
    return
  fi

  local git_clone_user_name
  local git_host
  local dev_root
  local worktrees_root
  local repos_root
  local sdks_root

  git_clone_user_name=$(ask_git_org)
  git_host=$(ask_git_host)
  dev_root=$(ask_dev_root)
  worktrees_root="$dev_root/worktrees"
  repos_root="$dev_root/repos"
  sdks_root="$dev_root/sdks"

  {
    printf "\nexport GIT_HOST=\"%s\"" "$git_host"
    printf "\nexport GIT_CLONE_USER_NAME=\"%s\"" "$git_clone_user_name"
    printf "\nexport PERSONAL_COMPUTER=\"%s\"" "$PERSONAL_COMPUTER"
    printf "\nexport DEV_ROOT=\"%s\"" "$dev_root"
    printf "\nexport WORKTREES_ROOT=\"%s\"" "$worktrees_root"
    printf "\nexport REPOS_ROOT=\"%s\"" "$repos_root"
    printf "\nexport SDKS_ROOT=\"%s\"" "$sdks_root"
    printf "\n"
    printf "source \"\$HOME/.melvin/config/base.zshrc\""
    printf "\n"
  } > "$ZSHRC"
}

# =============================================================================
# Gitconfig Functions
# =============================================================================

setup_gitconfig() {
  if [ -e "$GITCFG" ]; then
    return
  fi

  {
    printf "[include]\n\tpath = \"%s/.melvin/config/.gitconfig\"" "$HOME"

    if [ "$PERSONAL_COMPUTER" = true ]; then
      printf "\n\n[user]\n\temail = noreply@melvin.la"
      printf "\n\tname = Melvin"
      printf "\n\tsigningkey = 2C307E0D0413344B"
    else
      printf "\n\n[user]\n\temail = melvin@domain.tld"
      printf "\n\tname = Melvin"
      printf "\n\t# signingkey = <key>"
    fi

    printf "\n\n# [url \"ssh://git@github.com/\"]"
    printf "\n\t# insteadOf = https://github.com/"

    if [ "$PERSONAL_COMPUTER" = true ]; then
      printf "\n\n[commit]"
      printf "\n\tgpgsign = true"
    else
      printf "\n\n[commit]"
      printf "\n\tgpgsign = false"
    fi
  } > "$GITCFG"
}

# =============================================================================
# GPG Functions
# =============================================================================

setup_gpg() {
  # https://dev.to/wes/how2-using-gpg-on-macos-without-gpgtools-428f
  if [ -e "$HOME/.gnupg/gpg-agent.conf" ]; then
    return
  fi

  mkdir -p "$HOME/.gnupg"
  local pinentry_path="$HOMEBREW_PREFIX/bin/pinentry-mac"
  echo "pinentry-program $pinentry_path" > "$HOME/.gnupg/gpg-agent.conf"
  killall gpg-agent
  gpg-agent --daemon
}

# =============================================================================
# GitHub Functions
# =============================================================================

setup_github() {
  SETUP_GITHUB=false

  if [ "$GH_STATUS" = "success" ]; then
    SETUP_GITHUB=true
    return
  fi

  if ask_yes_no "Setup Github"; then
    SETUP_GITHUB=true
    gh auth login -w  # will ask to upload the previously generated SSH key to Github
  fi
}

# =============================================================================
# Final Tasks
# =============================================================================

print_remaining_tasks() {
  echo "Things left to do:"

  if [ "$SETUP_GITHUB" = false ]; then
    printf "\t* Upload %s/.ssh/default to your Cloud VCS: 'pbcopy < %s/.ssh/default.pub'\n" "$HOME" "$HOME"
  fi

  if ! mdfind "kMDItemKind == 'Application'" | grep -iF "EasyRes" > /dev/null 2>&1; then
    printf "\t* (optional) Install EasyRes if needed: http://easyresapp.com\n"
  fi

  printf "\t* (optional) Import PGP Key from Enpass with 'gpg --import private.key'\n"
}

# =============================================================================
# Claude Code Config Sync
# =============================================================================
#
# Syncs ~/.claude/{settings.json,skills,agents,commands} with the repo copy.
# PERSONAL_COMPUTER=true  → symlink (repo IS the live config).
# PERSONAL_COMPUTER=false → copy. Subsequent runs do a git-based 3-way merge:
# the base for each comparison is the file content at the commit SHA we last
# synced from (stored in .claude/.sync-state/last-sync-commit). Only true
# divergence (both local and repo changed differently from base) prompts.
# Honor SKIP_CLAUDE_MERGE_PROMPTS=keep-local|take-remote|skip for non-interactive runs.

CLAUDE_REPO_DIR=""
CLAUDE_HOME_DIR=""
CLAUDE_STATE_DIR=""
CLAUDE_LAST_SYNC_FILE=""
CLAUDE_DECISIONS_FILE=""
CLAUDE_HAD_SKIPS=0
CLAUDE_DIR_NAMES=("skills" "agents" "commands")
CLAUDE_TOP_FILES=("CLAUDE.md" "RTK.md")

claude_init_paths() {
  CLAUDE_REPO_DIR="$CONFIG_DIR/.claude"
  CLAUDE_HOME_DIR="$HOME/.claude"
  CLAUDE_STATE_DIR="$CLAUDE_REPO_DIR/.sync-state"
  CLAUDE_LAST_SYNC_FILE="$CLAUDE_STATE_DIR/last-sync-commit"
  CLAUDE_DECISIONS_FILE="$CLAUDE_STATE_DIR/decisions.json"
  CLAUDE_HAD_SKIPS=0
}

claude_ensure_state_dir() {
  mkdir -p "$CLAUDE_STATE_DIR"
  if [ ! -e "$CLAUDE_STATE_DIR/README.md" ]; then
    cat > "$CLAUDE_STATE_DIR/README.md" <<'EOF'
This directory is gitignored and managed by install.sh.

- last-sync-commit : repo SHA at last successful Claude config sync. Used as
  the base for 3-way merges of settings.json and the skills/agents/commands
  directories.
- decisions.json   : remembered "always keep local" / "always take remote"
  choices per conflict path. Delete this file to clear all remembered choices.

Wiping this directory forces a re-prompt on every divergent key/file the
next time install.sh runs.
EOF
  fi
  if [ ! -e "$CLAUDE_DECISIONS_FILE" ]; then
    printf '%s\n' '{"version":1,"settings":{},"files":{}}' > "$CLAUDE_DECISIONS_FILE"
  fi
}

# Write a timestamped copy of $1 next to itself: <path>.YYYYMMDDHHMMSS.bkp
claude_backup_file() {
  local file_path="$1"
  if [ ! -e "$file_path" ]; then
    return 0
  fi
  local stamp
  stamp=$(date +%Y%m%d%H%M%S)
  cp -p "$file_path" "$file_path.$stamp.bkp"
  return 0
}

# Look up a remembered decision. Echoes "local" / "remote" / "" (none).
# Usage: claude_get_decision settings|files <key>
claude_get_decision() {
  local kind="$1" key="$2"
  if [ ! -e "$CLAUDE_DECISIONS_FILE" ]; then
    printf ""
    return 0
  fi
  jq -r --arg kind "$kind" --arg k "$key" '
    (.[$kind] // {}) | (.[$k] // "")
  ' "$CLAUDE_DECISIONS_FILE" 2>/dev/null || printf ""
  return 0
}

# Persist a decision for next time.
# Usage: claude_set_decision settings|files <key> local|remote
claude_set_decision() {
  local kind="$1" key="$2" choice="$3"
  local tmp
  tmp=$(mktemp)
  jq --arg kind "$kind" --arg k "$key" --arg v "$choice" '
    .[$kind] = ((.[$kind] // {}) | .[$k] = $v)
  ' "$CLAUDE_DECISIONS_FILE" > "$tmp" && mv "$tmp" "$CLAUDE_DECISIONS_FILE"
}

# Resolve a setting/file conflict via env override, decisions cache, or
# interactive prompt. Sets $CLAUDE_LAST_CHOICE to keep-local | take-remote | skip.
# Returning a value via a global lets callers avoid command substitution, so
# CLAUDE_HAD_SKIPS=1 actually propagates to the parent shell.
# Usage: claude_resolve_conflict settings|files <key> <header> <details_callback>
#   <details_callback> is a shell function name that prints base/local/remote
#   details (no args = summary, "diff" arg = unified diff).
CLAUDE_LAST_CHOICE=""
claude_resolve_conflict() {
  local kind="$1" key="$2" header="$3" details_fn="$4"
  CLAUDE_LAST_CHOICE=""

  case "$SKIP_CLAUDE_MERGE_PROMPTS" in
    keep-local)  printf "%s -> keep-local (env)\n"  "$header" >&2; CLAUDE_LAST_CHOICE="keep-local";  return 0 ;;
    take-remote) printf "%s -> take-remote (env)\n" "$header" >&2; CLAUDE_LAST_CHOICE="take-remote"; return 0 ;;
    skip)        printf "%s -> skip (env)\n"        "$header" >&2; CLAUDE_HAD_SKIPS=1; CLAUDE_LAST_CHOICE="skip"; return 0 ;;
  esac

  local cached=""
  cached=$(claude_get_decision "$kind" "$key")
  case "$cached" in
    local)  printf "%s -> keep-local (remembered)\n"  "$header" >&2; CLAUDE_LAST_CHOICE="keep-local";  return 0 ;;
    remote) printf "%s -> take-remote (remembered)\n" "$header" >&2; CLAUDE_LAST_CHOICE="take-remote"; return 0 ;;
  esac

  while true; do
    {
      printf "\n%s\n" "$header"
      "$details_fn"
      printf "\n"
      printf "  1) Keep local\n"
      printf "  2) Take remote\n"
      printf "  3) View diff\n"
      printf "  4) Keep local AND remember\n"
      printf "  5) Take remote AND remember\n"
      printf "  6) Skip (do not advance last-sync-commit)\n"
      printf "  (clear remembered choices: rm %s)\n" "$CLAUDE_DECISIONS_FILE"
    } >&2
    local answer=""
    read -r answer
    case ${answer:0:1} in
      "1") CLAUDE_LAST_CHOICE="keep-local";  return 0 ;;
      "2") CLAUDE_LAST_CHOICE="take-remote"; return 0 ;;
      "3") "$details_fn" diff >&2 ;;
      "4") claude_set_decision "$kind" "$key" "local";  CLAUDE_LAST_CHOICE="keep-local";  return 0 ;;
      "5") claude_set_decision "$kind" "$key" "remote"; CLAUDE_LAST_CHOICE="take-remote"; return 0 ;;
      "6") CLAUDE_HAD_SKIPS=1; CLAUDE_LAST_CHOICE="skip"; return 0 ;;
      *)   printf "Invalid value\n" >&2 ;;
    esac
  done
}

# Returns 0 (success) and echoes content of <repo>/.claude/<rel> at the last
# synced SHA, or returns 1 if no sync state exists or the file isn't there yet.
claude_show_base() {
  local rel="$1"
  if [ ! -e "$CLAUDE_LAST_SYNC_FILE" ]; then
    return 1
  fi
  local sha
  sha=$(cat "$CLAUDE_LAST_SYNC_FILE")
  if [ -z "$sha" ]; then
    return 1
  fi
  git -C "$CONFIG_DIR" show "$sha:.claude/$rel" 2>/dev/null
}

# Returns 0 if the file existed in the base tree, 1 otherwise.
claude_base_has() {
  local rel="$1"
  if [ ! -e "$CLAUDE_LAST_SYNC_FILE" ]; then
    return 1
  fi
  local sha
  sha=$(cat "$CLAUDE_LAST_SYNC_FILE")
  if [ -z "$sha" ]; then
    return 1
  fi
  git -C "$CONFIG_DIR" cat-file -e "$sha:.claude/$rel" 2>/dev/null
}

claude_advance_sync_commit() {
  if [ "$CLAUDE_HAD_SKIPS" = "1" ]; then
    printf "claude_setup: leaving last-sync-commit unchanged due to skipped conflicts\n" >&2
    return 0
  fi
  git -C "$CONFIG_DIR" rev-parse HEAD > "$CLAUDE_LAST_SYNC_FILE"
  return 0
}

# Render a path-array JSON to jq-filter syntax, e.g.
#   ["enabledPlugins","warp@x"] -> .enabledPlugins["warp@x"]
claude_path_to_filter() {
  jq -r '
    def step(k): if (k | type) == "string" and (k | test("^[a-zA-Z_][a-zA-Z_0-9]*$"))
                 then ".\(k)"
                 else ".[\(k | tojson)]" end;
    map(step(.)) | join("")
  ' <<< "$1"
}

# Compute merge decisions for settings.json. Reads $1 (base), $2 (local),
# $3 (remote) and emits a JSON array of {path, decision} objects on stdout,
# one entry per non-noop leaf path.
#
# Granularity is dynamic: if every present occurrence of a top-level key is an
# object, the merge unit is each (top-key, sub-key); otherwise the whole value.
#
# decision.action ∈ "take-remote" | "remote-delete" | "keep-local" | "conflict"
# For "take-remote", decision.value carries the value to set.
# For "conflict", decision.{base,local,remote} are {value:...} or null (absent).
claude_compute_settings_decisions() {
  local base_file="$1" local_file="$2" remote_file="$3"
  jq -nc \
    --slurpfile base   "$base_file" \
    --slurpfile local  "$local_file" \
    --slurpfile remote "$remote_file" '
    def lookup($obj; $p):
      if ($p | length) == 0 then {present: true, value: $obj}
      elif ($obj | type) != "object" or ($obj | has($p[0]) | not) then {present: false}
      else lookup($obj[$p[0]]; $p[1:])
      end;
    def equal($x; $y):
      ($x.present == $y.present)
      and (($x.present | not) or ($x.value == $y.value));
    def decide($b; $l; $r):
      if equal($b; $l) and equal($b; $r) then {action: "noop"}
      elif equal($b; $l) then
        if $r.present then {action: "take-remote", value: $r.value}
        else {action: "remote-delete"} end
      elif equal($b; $r) then {action: "keep-local"}
      elif equal($l; $r) then
        if $l.present then {action: "noop"}
        else {action: "remote-delete"} end
      else
        {action: "conflict",
         base:   (if $b.present then {value: $b.value} else null end),
         local:  (if $l.present then {value: $l.value} else null end),
         remote: (if $r.present then {value: $r.value} else null end)}
      end;

    ($base[0]   // {}) as $B |
    ($local[0]  // {}) as $L |
    ($remote[0] // {}) as $R |

    # Top-level keys that are present in any of B/L/R
    [$B, $L, $R | select(type == "object") | keys_unsorted] | add | unique | sort as $tk |

    [ $tk[] as $k
      | (
          # types of this key across the three (only where present)
          [ $B, $L, $R
            | select(type == "object" and has($k))
            | .[$k] | type
          ] | unique
        ) as $types
      | if $types == ["object"] then
          ( [ $B, $L, $R
              | select(type == "object" and has($k) and (.[$k] | type) == "object")
              | .[$k] | keys_unsorted
            ] | add | unique | sort | .[] ) as $sk
          | [$k, $sk]
        else [$k]
        end
    ]
    | [ .[] as $p
        | {path: $p, decision: decide(lookup($B; $p); lookup($L; $p); lookup($R; $p))}
        | select(.decision.action != "noop")
      ]
  '
}

# Conflict-detail callback for claude_resolve_conflict (settings.json).
# Reads from CLAUDE_CONFLICT_* globals set by claude_merge_settings.
CLAUDE_CONFLICT_BASE_REPR=""
CLAUDE_CONFLICT_LOCAL_REPR=""
CLAUDE_CONFLICT_REMOTE_REPR=""
CLAUDE_CONFLICT_BASE_SHA=""
claude_settings_conflict_details() {
  local mode="${1:-summary}"
  if [ "$mode" = "diff" ]; then
    diff -u <(printf '%s\n' "$CLAUDE_CONFLICT_BASE_REPR") \
            <(printf '%s\n' "$CLAUDE_CONFLICT_LOCAL_REPR") | sed 's/^/    /' || true
    diff -u <(printf '%s\n' "$CLAUDE_CONFLICT_LOCAL_REPR") \
            <(printf '%s\n' "$CLAUDE_CONFLICT_REMOTE_REPR") | sed 's/^/    /' || true
  else
    printf "  base   (sha %s): %s\n" "${CLAUDE_CONFLICT_BASE_SHA:-none}" "$CLAUDE_CONFLICT_BASE_REPR"
    printf "  local                : %s\n" "$CLAUDE_CONFLICT_LOCAL_REPR"
    printf "  remote               : %s\n" "$CLAUDE_CONFLICT_REMOTE_REPR"
  fi
}

# 3-way merge of settings.json. Reads base from git, local from disk, remote
# from the repo working tree. Writes merged result over local with backup.
claude_merge_settings() {
  local local_file="$CLAUDE_HOME_DIR/settings.json"
  local remote_file="$CLAUDE_REPO_DIR/settings.json"

  if [ ! -e "$remote_file" ]; then
    return 0
  fi

  if ! jq empty "$remote_file" 2>/dev/null; then
    printf "claude_merge_settings: %s is not valid JSON, skipping\n" "$remote_file" >&2
    return 0
  fi

  if [ -e "$local_file" ] && ! jq empty "$local_file" 2>/dev/null; then
    printf "claude_merge_settings: %s is not valid JSON, refusing to merge (fix or remove the file and re-run)\n" "$local_file" >&2
    return 0
  fi

  # Fast path: no local file yet (and we get to start fresh) → just copy the
  # remote, preserving its exact formatting and key order. Avoids the merge
  # output's alphabetical sort.
  if [ ! -e "$local_file" ]; then
    cp "$remote_file" "$local_file"
    return 0
  fi

  local tmp_base tmp_local ops_file
  tmp_base=$(mktemp)
  tmp_local=$(mktemp)
  ops_file=$(mktemp)

  # Resolve base: prefer git, fall back to empty object on first sync or
  # when last-sync-commit references a SHA we can't read.
  if claude_show_base "settings.json" > "$tmp_base" 2>/dev/null && [ -s "$tmp_base" ] && jq empty "$tmp_base" 2>/dev/null; then
    :
  else
    printf '{}' > "$tmp_base"
    if [ -e "$CLAUDE_LAST_SYNC_FILE" ]; then
      printf "claude_merge_settings: cannot read settings.json at last-sync-commit, treating base as empty\n" >&2
    fi
  fi

  if [ -e "$local_file" ]; then
    cp "$local_file" "$tmp_local"
  else
    printf '{}' > "$tmp_local"
  fi

  CLAUDE_CONFLICT_BASE_SHA=""
  if [ -e "$CLAUDE_LAST_SYNC_FILE" ]; then
    CLAUDE_CONFLICT_BASE_SHA=$(git -C "$CONFIG_DIR" rev-parse --short "$(cat "$CLAUDE_LAST_SYNC_FILE")" 2>/dev/null || echo "")
  fi

  local decisions
  decisions=$(claude_compute_settings_decisions "$tmp_base" "$tmp_local" "$remote_file") || {
    printf "claude_merge_settings: jq merge computation failed\n" >&2
    rm -f "$tmp_base" "$tmp_local" "$ops_file"
    return 1
  }

  local count
  count=$(jq 'length' <<< "$decisions")
  local i=0
  while [ "$i" -lt "$count" ]; do
    local entry json_path action
    entry=$(jq -c ".[$i]" <<< "$decisions")
    json_path=$(jq -c '.path' <<< "$entry")
    action=$(jq -r '.decision.action' <<< "$entry")

    case "$action" in
      "take-remote")
        local value
        value=$(jq -c '.decision.value' <<< "$entry")
        jq -nc --argjson p "$json_path" --argjson v "$value" '{path: $p, value: $v}' >> "$ops_file"
        ;;
      "remote-delete")
        jq -nc --argjson p "$json_path" '{path: $p, delete: true}' >> "$ops_file"
        ;;
      "keep-local")
        : # disk already has the right value
        ;;
      "conflict")
        local key filter header
        key="$json_path"  # canonical compact-JSON key for the cache
        filter=$(claude_path_to_filter "$json_path")
        header="Conflict in settings.json at $filter"

        CLAUDE_CONFLICT_BASE_REPR=$(jq -r 'if .decision.base   == null then "<absent>" else (.decision.base.value   | tojson) end' <<< "$entry")
        CLAUDE_CONFLICT_LOCAL_REPR=$(jq -r 'if .decision.local  == null then "<absent>" else (.decision.local.value  | tojson) end' <<< "$entry")
        CLAUDE_CONFLICT_REMOTE_REPR=$(jq -r 'if .decision.remote == null then "<absent>" else (.decision.remote.value | tojson) end' <<< "$entry")

        claude_resolve_conflict "settings" "$key" "$header" claude_settings_conflict_details
        case "$CLAUDE_LAST_CHOICE" in
          "keep-local") ;;
          "take-remote")
            local r_present
            r_present=$(jq -r '.decision.remote != null' <<< "$entry")
            if [ "$r_present" = "true" ]; then
              local r_value
              r_value=$(jq -c '.decision.remote.value' <<< "$entry")
              jq -nc --argjson p "$json_path" --argjson v "$r_value" '{path: $p, value: $v}' >> "$ops_file"
            else
              jq -nc --argjson p "$json_path" '{path: $p, delete: true}' >> "$ops_file"
            fi
            ;;
          "skip") : ;;
        esac
        ;;
    esac

    i=$((i + 1))
  done

  if [ -s "$ops_file" ]; then
    claude_backup_file "$local_file"
    local out_tmp="$local_file.tmp.$$"
    jq --slurpfile ops "$ops_file" '
      reduce $ops[] as $op (.;
        if ($op | has("delete")) then delpaths([$op.path])
        else setpath($op.path; $op.value) end)
    ' "$tmp_local" > "$out_tmp"
    mv "$out_tmp" "$local_file"
  fi

  rm -f "$tmp_base" "$tmp_local" "$ops_file"
}

# Symlink mode: ~/.claude/{settings.json,CLAUDE.md,RTK.md,skills,agents,commands} -> repo copies.
claude_install_symlink() {
  local items=("settings.json" "${CLAUDE_TOP_FILES[@]}" "${CLAUDE_DIR_NAMES[@]}")
  local item source target
  for item in "${items[@]}"; do
    source="$CLAUDE_REPO_DIR/$item"
    target="$CLAUDE_HOME_DIR/$item"

    if [ ! -e "$source" ]; then
      continue
    fi

    # Already the correct symlink? Skip silently.
    if [ -L "$target" ] && [ "$(readlink "$target")" = "$source" ]; then
      continue
    fi

    if [ -e "$target" ] || [ -L "$target" ]; then
      if [ "$SKIP_CONFIG_FILE_SETUP" = true ]; then
        printf "Skipping existing %s\n" "$target"
      else
        handle_existing_file "$source" "$target"
      fi
    else
      ln -s "$source" "$target"
    fi
  done
  return 0
}

# Conflict-detail callback for non-JSON file conflicts (top-level files and
# skill/agent/command files). Globals set by claude_merge_file before invoking
# the resolver.
CLAUDE_DIR_CONFLICT_TYPE=""
CLAUDE_DIR_CONFLICT_LOCAL_PATH=""
CLAUDE_DIR_CONFLICT_REMOTE_PATH=""
CLAUDE_DIR_CONFLICT_BASE_PATH=""
claude_dir_conflict_details() {
  local mode="${1:-summary}"
  local sha_label="${CLAUDE_CONFLICT_BASE_SHA:-none}"
  if [ "$mode" = "diff" ]; then
    local lhs="/dev/null" rhs="/dev/null"
    [ -n "$CLAUDE_DIR_CONFLICT_LOCAL_PATH"  ] && lhs="$CLAUDE_DIR_CONFLICT_LOCAL_PATH"
    [ -n "$CLAUDE_DIR_CONFLICT_REMOTE_PATH" ] && rhs="$CLAUDE_DIR_CONFLICT_REMOTE_PATH"
    diff -u "$lhs" "$rhs" | sed 's/^/    /' || true
  else
    local sz
    if [ -n "$CLAUDE_DIR_CONFLICT_BASE_PATH" ]; then
      sz=$(wc -c < "$CLAUDE_DIR_CONFLICT_BASE_PATH" 2>/dev/null | tr -d ' ' || echo "?")
      printf "  base   (sha %s): present (%s bytes)\n" "$sha_label" "$sz"
    else
      printf "  base   (sha %s): absent\n" "$sha_label"
    fi
    if [ -n "$CLAUDE_DIR_CONFLICT_LOCAL_PATH" ]; then
      sz=$(wc -c < "$CLAUDE_DIR_CONFLICT_LOCAL_PATH" 2>/dev/null | tr -d ' ' || echo "?")
      printf "  local                : present (%s bytes)\n" "$sz"
    else
      printf "  local                : absent\n"
    fi
    if [ -n "$CLAUDE_DIR_CONFLICT_REMOTE_PATH" ]; then
      sz=$(wc -c < "$CLAUDE_DIR_CONFLICT_REMOTE_PATH" 2>/dev/null | tr -d ' ' || echo "?")
      printf "  remote               : present (%s bytes)\n" "$sz"
    else
      printf "  remote               : absent\n"
    fi
    printf "  conflict type        : %s\n" "$CLAUDE_DIR_CONFLICT_TYPE"
  fi
}

# Merge a single file by its path within .claude/ (e.g. "CLAUDE.md" or
# "skills/foo.md"). Resolves the file-level truth table and applies the chosen
# action, always backing up local content before any destructive change.
claude_merge_file() {
  local rel="$1"
  local L="$CLAUDE_HOME_DIR/$rel"
  local R="$CLAUDE_REPO_DIR/$rel"

  local has_L=0 has_R=0 has_B=0
  if [ -e "$L" ]; then has_L=1; fi
  if [ -e "$R" ]; then has_R=1; fi

  local tmp_base=""
  if claude_base_has "$rel"; then
    has_B=1
    tmp_base=$(mktemp)
    claude_show_base "$rel" > "$tmp_base" 2>/dev/null
  fi

  local L_eq_B=0 R_eq_B=0 L_eq_R=0
  if [ "$has_L" = "1" ] && [ "$has_B" = "1" ] && cmp -s "$L" "$tmp_base"; then L_eq_B=1; fi
  if [ "$has_R" = "1" ] && [ "$has_B" = "1" ] && cmp -s "$R" "$tmp_base"; then R_eq_B=1; fi
  if [ "$has_L" = "1" ] && [ "$has_R" = "1" ] && cmp -s "$L" "$R";        then L_eq_R=1; fi

  local action="noop" conflict_type=""

  if [ "$has_L" = 1 ] && [ "$has_R" = 1 ] && [ "$has_B" = 1 ]; then
    if   [ "$L_eq_B" = 1 ] && [ "$R_eq_B" = 1 ]; then action="noop"
    elif [ "$L_eq_B" = 1 ];                       then action="copy-r-to-l"
    elif [ "$R_eq_B" = 1 ];                       then action="noop"
    elif [ "$L_eq_R" = 1 ];                       then action="noop"
    else action="conflict"; conflict_type="modify-modify"
    fi
  elif [ "$has_L" = 1 ] && [ "$has_R" = 1 ] && [ "$has_B" = 0 ]; then
    if [ "$L_eq_R" = 1 ]; then action="noop"
    else action="conflict"; conflict_type="add-add-diff"
    fi
  elif [ "$has_L" = 1 ] && [ "$has_R" = 0 ] && [ "$has_B" = 1 ]; then
    if [ "$L_eq_B" = 1 ]; then action="rm-l"
    else action="conflict"; conflict_type="modify-delete"
    fi
  elif [ "$has_L" = 1 ] && [ "$has_R" = 0 ] && [ "$has_B" = 0 ]; then
    action="noop"  # local-add (only on disk, never seen by repo)
  elif [ "$has_L" = 0 ] && [ "$has_R" = 1 ] && [ "$has_B" = 1 ]; then
    if [ "$R_eq_B" = 1 ]; then action="noop"  # local-delete clean
    else action="conflict"; conflict_type="delete-modify"
    fi
  elif [ "$has_L" = 0 ] && [ "$has_R" = 1 ] && [ "$has_B" = 0 ]; then
    action="copy-r-to-l"  # remote-add
  fi

  case "$action" in
    "noop") : ;;
    "copy-r-to-l")
      if [ -e "$L" ]; then claude_backup_file "$L"; fi
      mkdir -p "$(dirname "$L")"
      cp -p "$R" "$L"
      ;;
    "rm-l")
      claude_backup_file "$L"
      rm -f "$L"
      ;;
    "conflict")
      local key="$rel"
      local header="Conflict in $rel ($conflict_type)"

      CLAUDE_DIR_CONFLICT_TYPE="$conflict_type"
      CLAUDE_DIR_CONFLICT_LOCAL_PATH=""
      CLAUDE_DIR_CONFLICT_REMOTE_PATH=""
      CLAUDE_DIR_CONFLICT_BASE_PATH="$tmp_base"
      if [ "$has_L" = 1 ]; then CLAUDE_DIR_CONFLICT_LOCAL_PATH="$L"; fi
      if [ "$has_R" = 1 ]; then CLAUDE_DIR_CONFLICT_REMOTE_PATH="$R"; fi

      claude_resolve_conflict "files" "$key" "$header" claude_dir_conflict_details
      case "$CLAUDE_LAST_CHOICE" in
        "keep-local") ;;
        "take-remote")
          # "Take remote" means: make L look like R (may be absent → delete)
          if [ "$has_R" = 1 ]; then
            if [ -e "$L" ]; then claude_backup_file "$L"; fi
            mkdir -p "$(dirname "$L")"
            cp -p "$R" "$L"
          else
            if [ -e "$L" ]; then claude_backup_file "$L"; fi
            rm -f "$L"
          fi
          ;;
        "skip") : ;;
      esac
      ;;
  esac

  if [ -n "$tmp_base" ]; then rm -f "$tmp_base"; fi
  return 0
}

# Walk the union of files in $CLAUDE_HOME_DIR/$dir, $CLAUDE_REPO_DIR/$dir and
# the base tree, then merge each file. .gitkeep entries are ignored — they
# exist only to keep empty repo directories tracked by git.
claude_merge_dir() {
  local dir_name="$1"
  local local_dir="$CLAUDE_HOME_DIR/$dir_name"
  local remote_dir="$CLAUDE_REPO_DIR/$dir_name"

  mkdir -p "$local_dir"

  local tmp_files
  tmp_files=$(mktemp)

  if [ -d "$remote_dir" ]; then
    ( cd "$remote_dir" && find . -type f ! -name .gitkeep | sed 's|^\./||' ) >> "$tmp_files"
  fi
  if [ -d "$local_dir" ]; then
    ( cd "$local_dir"  && find . -type f ! -name .gitkeep | sed 's|^\./||' ) >> "$tmp_files"
  fi
  if [ -e "$CLAUDE_LAST_SYNC_FILE" ]; then
    local sha
    sha=$(cat "$CLAUDE_LAST_SYNC_FILE")
    if [ -n "$sha" ]; then
      git -C "$CONFIG_DIR" ls-tree -r --name-only "$sha" -- ".claude/$dir_name" 2>/dev/null \
        | sed "s|^\\.claude/$dir_name/||" \
        | grep -v '^\.gitkeep$' >> "$tmp_files" || true
    fi
  fi

  local rels
  rels=$(sort -u "$tmp_files")
  rm -f "$tmp_files"

  if [ -z "$rels" ]; then
    return 0
  fi

  while IFS= read -r rel; do
    [ -n "$rel" ] || continue
    claude_merge_file "$dir_name/$rel"
  done <<< "$rels"
}

# Copy mode entry point: settings.json merge + each top-level file merge +
# each directory merge, then advance last-sync-commit (only if no conflicts
# were skipped).
claude_install_or_merge_copy() {
  claude_merge_settings
  local f
  for f in "${CLAUDE_TOP_FILES[@]}"; do
    claude_merge_file "$f"
  done
  local d
  for d in "${CLAUDE_DIR_NAMES[@]}"; do
    claude_merge_dir "$d"
  done
  claude_advance_sync_commit
}

claude_setup() {
  command -v jq  >/dev/null 2>&1 || { printf "claude_setup: jq not found, skipping\n"  >&2; return 0; }
  command -v git >/dev/null 2>&1 || { printf "claude_setup: git not found, skipping\n" >&2; return 0; }

  claude_init_paths

  if [ ! -d "$CLAUDE_REPO_DIR" ]; then
    printf "claude_setup: %s does not exist, skipping\n" "$CLAUDE_REPO_DIR" >&2
    return 0
  fi

  mkdir -p "$CLAUDE_HOME_DIR"

  if [ "$PERSONAL_COMPUTER" = "true" ]; then
    claude_install_symlink
  else
    claude_ensure_state_dir
    claude_install_or_merge_copy
  fi
}

# =============================================================================
# Main
# =============================================================================

main() {
  # Initial setup prompts
  setup_personal_computer_flag
  setup_skip_existing_flag

  # If the repo already exists, pull first so the latest install.sh is used.
  # If new commits were fetched, re-exec this script with the updated version.
  if [ -d "$CONFIG_DIR" ]; then
    update_config_repo || exec env \
      PERSONAL_COMPUTER="$PERSONAL_COMPUTER" \
      SKIP_CONFIG_FILE_SETUP="$SKIP_CONFIG_FILE_SETUP" \
      SKIP_CLAUDE_MERGE_PROMPTS="$SKIP_CLAUDE_MERGE_PROMPTS" \
      zsh "$CONFIG_DIR/install.sh"
  fi

  # Install dependencies
  install_homebrew
  upgrade_homebrew_packages
  install_packages

  # Setup SSH
  setup_ssh

  # Get GitHub status for later use
  GH_STATUS=$(gh auth status -a --json hosts --jq '.hosts["github.com"][0].state')
  GH_ACCOUNT=$(gh auth status -a --json hosts --jq '.hosts["github.com"][0].login')

  # Setup config repository
  if [ ! -d "$CONFIG_DIR" ]; then
    clone_config_repo
  fi

  # Copy config files
  copy_config_files

  # Setup shell and git configs
  setup_zshrc
  setup_gitconfig

  # Setup GPG
  setup_gpg

  # Setup GitHub
  setup_github
 
  # Update last check timestamp
  date +%s > "$CONFIG_DIR/.last_update_check"

  # Print remaining manual tasks
  print_remaining_tasks
  reload_zshrc
}

reload_zshrc() {
  source "$HOME/.zshrc"
}

# Run main function
main
