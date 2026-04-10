#!/bin/env zsh

# Shared
export PATH=$HOME/.bin-remote:$PATH

# All the stuff installed by brew, that needs their bin directories
# to be added to PATH
brew_installed=(
    'rustup' # rust
    # libpq conflicts with the package postgres.
    # libpq only has client tools (psql, pq_dump, etc) while postgres
    # has everything
    'libpq'
)
for package in $brew_installed; do
  export PATH="$(brew --prefix $package)/bin:$PATH"
done

# Go
mkdir -p $HOME/.go
export GOPATH=$HOME/.go
export PATH=$PATH:$GOPATH/bin

# GNU grep
if [ -d "$(brew --prefix grep)" ]; then
    export PATH="$(brew --prefix grep)/libexec/gnubin:$PATH"
fi

# google cloud
# https://cloud.google.com/sdk/docs/install
if [ -d "$HOME/google-cloud-sdk" ]; then
    export PATH="$HOME/google-cloud-sdk/bin:$PATH"
fi

# Node
mkdir -p $HOME/.nvm
export NVM_DIR=~/.nvm
if [ -e "$(brew --prefix nvm)/nvm.sh" ]; then
  source $(brew --prefix nvm)/nvm.sh
fi

alias emacs='\emacs -nw'
alias cd..='cd ..'
alias lss='less'
alias grep='grep --color'
alias rm='rm -i'
alias reload=". $HOME/.zshrc"
alias extract-pkg="pkgutil --expand-full " # usage extract-pkg [pkg] [out_dir]

alias lsd='lsd --config-file="$HOME/.melvin/config/lsd.yaml"'
alias ls='lsd -hF'
alias lt='lsd --tree'
alias ll='lsd -l'
alias lla='lsd -la'
alias lla='lsd -la'
alias la='lsd -a'
alias prs='gh pr list --author "@me"'
alias pr='gh pr'

export EDITOR='emacs'
export PAGER='less'
export LESS="R --quit-if-one-screen"
export CLICOLOR=1

# record terminal to file
function rec {
    local dir="$HOME/Documents/term_rec"

    if [ -z "$1" ]; then
        echo "usage: rec [-adkpqr] [-F pipe] [-t time] output-file-name"
    fi

    local flags
    local outfile=$1

    if [ "${#@[*]}" -gt "1" ]; then
        flags="${@:1:$#@-1}"
        outfile=${*:$#}
    fi

    mkdir -p "$dir"

    script $flags "$dir/$outfile"
}

function findport {
    if [ -z "$1" ]; then
        echo "usage: findport port-number"
    fi

    lsof -nP -i4TCP:$1 | grep LISTEN
}

function erase {
    if [ -z "$1" ]; then
        echo "usage: erase _directory_"
    fi

    mkdir -p trash

    # mkdir -p trash ; rsync -a --delete trash/ "$1" && rmdir trash
    for dir in "$@"
    do
       rsync -aP --delete trash/ "$dir"
       rmdir "$dir"
    done

    rm -rf trash
}

function code {
    if [[ "$(command -v code)" == /* ]]; then
        command code "$@"
    else
        code-insiders "$@"
    fi
}

function is-go-repo {
    return $(test -e "go.mod")
}

# TODO(melvin): figure out a clean not-too-hacky way to clean that up
function run {
    if [ -e ".nvmrc" ]; then
        nvm install
    fi

    if [ -e "yarn.lock" ]; then
        yarn "$@"
    elif [ -e "pnpm-lock.yaml" ]; then
        pnpm "$@"
    elif [ -e "nx.json" ]; then
        nx "$@"
    elif [ -e "bun.lock" ]; then
        bun run "$@"
    elif [ -e "package-lock.json" ]; then
        npm run "$@"
    elif [ -e "Makefile" ]; then
        make "$@"
    elif is-go-repo; then
        task "$@"
    elif [ -e "manage.py" ]; then
        python manage.py "$@"
    else
        echo "Nothing to run"
        return 1
    fi
}

function add {
    if [ -e "yarn.lock" ]; then
        yarn add "$@"
    elif [ -e "pnpm-lock.yaml" ]; then
        pnpm add "$@"
    elif [ -e "bun.lock" ]; then
        bun add "$@"
    elif [ -e "package-lock.json" ]; then
        npm install "$@"
    elif is-go-repo; then
        go get "$@"
    elif [ -d ".venv" ]; then
        pip install "$@"
        pip freeze > requirements.txt
    else
        echo "Nothing to run"
        return 1
    fi
}

function install {
    if [ -e "yarn.lock" ]; then
        yarn install
    elif [ -e "pnpm-lock.yaml" ]; then
        pnpm install
    elif [ -e "bun.lock" ]; then
        bun install
    elif [ -e "package-lock.json" ]; then
        npm install
    elif [ -e "go.mod" ]; then
        go mod tidy
    elif [ -d ".venv" ]; then
        pip freeze > requirements.txt
    else
        echo "Nothing to run"
        return 1
    fi
}

function lint {
    if [ -e "Makefile" ]; then
        make lint "$@"
    elif is-go-repo; then
        golangci-lint run ./... "$@"
    else
        run lint "$@"
    fi
}

function cl() {
    if [ -z "$1" ]; then
        echo "cl <repo-name | user/repo-name>"
    fi

    local host="${GIT_HOST:-git@github.com}"
    local user="${GIT_CLONE_USER_NAME:-Nivl}"

    local repo="$user/$1"
    if [[ "$1" =~ '/' ]]; then
         repo="$1"
    fi

    git clone "$host:$repo.git"
    cd "$1"
}

function keygen() {
    local size="${1:-32}"

    print $(cat /dev/urandom | LC_ALL=C tr -dc 'a-zA-Z0-9' | fold -w "$size" | head -n 1)
}

function _should_copy_wt_ignored_path() {
  local path="${1%/}"

  case "$path" in
    .git|.git/*|*/.git|*/.git/*)
      return 1
      ;;
    .cache|.cache/*|*/.cache|*/.cache/*)
      return 1
      ;;
    .next|.next/*|*/.next|*/.next/*)
      return 1
      ;;
    .turbo|.turbo/*|*/.turbo|*/.turbo/*)
      return 1
      ;;
    .pnpm-store|.pnpm-store/*|*/.pnpm-store|*/.pnpm-store/*)
      return 1
      ;;
    .yarn/cache|.yarn/cache/*|*/.yarn/cache|*/.yarn/cache/*)
      return 1
      ;;
    coverage|coverage/*|*/coverage|*/coverage/*)
      return 1
      ;;
    dist|dist/*|*/dist|*/dist/*)
      return 1
      ;;
    build|build/*|*/build|*/build/*)
      return 1
      ;;
    out|out/*|*/out|*/out/*)
      return 1
      ;;
    tmp|tmp/*|*/tmp|*/tmp/*|temp|temp/*|*/temp|*/temp/*)
      return 1
      ;;
  esac

  return 0
}

function wt() {
  if [ -z "$1" ]; then
      echo "wt <feature-name>"
      return 1
  fi

  local feature_name="$1"
  local project_dir=$(git rev-parse --show-toplevel) || return 1

  # We put the worktree in a folder named after the repo's origin URL,
  # to avoid conflicts between repos with the same name.
  local wk_root="${WORKTREES_ROOT:-"$HOME/.wt"}"
  local remote_url=$(git -C "$project_dir" config --get remote.origin.url) || return 1
  local wk_project_path="$remote_url"

  case "$wk_project_path" in
    https://*|http://*)
      wk_project_path="${wk_project_path#https://}"
      wk_project_path="${wk_project_path#http://}"
      ;;
    git@*:* )
      wk_project_path="${wk_project_path#git@}"
      wk_project_path="${wk_project_path/:/\/}"
      ;;
  esac

  wk_project_path="${wk_project_path%.git}"

  local worktree_parent="$wk_root/$wk_project_path"
  mkdir -p "$worktree_parent"

  # Define the full path of the new worktree folder
  local worktree_path="${worktree_parent}/${feature_name}"

  # Create the worktree and the branch
  git -C "$project_dir" worktree add -b "$feature_name" "$worktree_path"

  local ignored_path
  while IFS= read -r ignored_path; do
    local relative_path="${ignored_path%/}"

    [ -n "$relative_path" ] || continue

    if ! _should_copy_wt_ignored_path "$relative_path"; then
      continue
    fi

    (
      cd "$project_dir" || exit 1
      rsync -aR -- "$relative_path" "$worktree_path"
    ) || return 1
  done < <(
    git -C "$project_dir" ls-files --others --ignored --exclude-standard --directory --no-empty-directory
  )

  cd "$worktree_path"
  code . &

  echo "Ready to work."
}

function _confirm_wt_done_delete() {
  local answer

  while true; do
    echo "Worktree has uncommitted or unpushed changes."
    read "answer?Type 'continue' to force delete or 'cancel' to abort: "

    case "$answer" in
      continue)
        return 0
        ;;
      cancel|"")
        echo "Canceled wt-done."
        return 1
        ;;
      *)
        echo "Please type 'continue' or 'cancel'."
        ;;
    esac
  done
}

function wt_done() {
  local project_dir=$(git rev-parse --show-toplevel 2>/dev/null) || {
    echo "wt-done must be run inside a Git repository."
    return 1
  }
  local wk_root="${WORKTREES_ROOT:-"$HOME/.wt"}"
  local git_dir=$(git rev-parse --git-dir) || return 1
  local common_dir=$(git rev-parse --git-common-dir) || return 1

  if [ "$git_dir" = "$common_dir" ]; then
    echo "wt-done only works inside a linked worktree."
    return 1
  fi

  local branch_name=$(git symbolic-ref --quiet --short HEAD) || {
    echo "wt-done could not determine the current branch."
    return 1
  }
  local has_uncommitted=false
  local has_unpushed=false

  if [ -n "$(git status --porcelain --untracked-files=normal)" ]; then
    has_uncommitted=true
  fi

  if git rev-parse --verify --quiet '@{upstream}' >/dev/null; then
    if [ "$(git rev-list --count '@{upstream}..HEAD')" -gt 0 ]; then
      has_unpushed=true
    fi
  else
    has_unpushed=true
  fi


  local redirect_target="${common_dir%.git}"
  if [ ! -d "$redirect_target" ]; then
    redirect_target="${REPOS_ROOT:-$HOME}"
  fi

  local -a remove_args
  if $has_uncommitted || $has_unpushed; then
    _confirm_wt_done_delete || return 1
    remove_args=(--force)
  else
    remove_args=()
  fi

  cd "$redirect_target" || return 1
  git --git-dir="$common_dir" worktree remove "${remove_args[@]}" "$project_dir" || return 1
  git --git-dir="$common_dir" branch -D "$branch_name" || return 1

  echo "Deleted worktree '$project_dir' and branch '$branch_name'."
}

alias wt-done='wt_done'
