#!/bin/bash

set -e  # Exit on error

# =============================================================================
# Configuration
# =============================================================================

CONFIG_DIR="$HOME/.melvin/config"
ZSHRC="$HOME/.zshrc"
GITCFG="$HOME/.gitconfig"

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
  local i=0

  if [ "${#FAILED_CASK_UPDATES[@]}" -eq 0 ]; then
    return
  fi

  printf "\nFailed cask installs or upgrades:\n"
  for i in "${!FAILED_CASK_UPDATES[@]}"; do
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
  brew install copilot-cli claude-code

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
    return
  fi

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
}

setup_config_repo() {
  if [ ! -d "$CONFIG_DIR" ]; then
    clone_config_repo
  else
    update_config_repo
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
}

# =============================================================================
# Zshrc Functions
# =============================================================================

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

  git_clone_user_name=$(ask_git_org)
  git_host=$(ask_git_host)

  {
    printf "source \"\$HOME/.melvin/config/base.zshrc\""
    printf "\n"
    printf "\nexport GIT_HOST=\"%s\"" "$git_host"
    printf "\nexport GIT_CLONE_USER_NAME=\"%s\"" "$git_clone_user_name"
    printf "\nexport PERSONAL_COMPUTER=\"%s\"" "$PERSONAL_COMPUTER"
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
# Main
# =============================================================================

main() {
  # Initial setup prompts
  setup_personal_computer_flag
  setup_skip_existing_flag

  # Install dependencies
  install_homebrew
  install_packages

  # Setup SSH
  setup_ssh

  # Get GitHub status for later use
  GH_STATUS=$(gh auth status -a --json hosts --jq '.hosts["github.com"][0].state')
  GH_ACCOUNT=$(gh auth status -a --json hosts --jq '.hosts["github.com"][0].login')

  # Setup config repository
  setup_config_repo

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
}

# Run main function
main
