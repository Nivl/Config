#!/bin/env zsh
# ZSH conf

#
# Requirements:
# brew install less
#

BREW_PATH="/opt/Homebrew"
if [ ! -d "/opt/Homebrew" ]; then
    BREW_PATH="/usr/local"
fi

zstyle ':completion:*' special-dirs true

export ZSH=$HOME/.oh-my-zsh
ZSH_THEME="nivl"
HIST_STAMPS="mm/dd/yyyy"
DISABLE_LS_COLORS="false"
plugins=(brew history extract npm docker docker-compose pod encode64 urltools)
DEFAULT_USER=melvin
# enable command auto-correction (annoying because of "git co" being corrected as "git ci", etc.)
# ENABLE_CORRECTION="true"
# COMPLETION_WAITING_DOTS="true"
# uncomment to make VCS much faster on big repo
# DISABLE_UNTRACKED_FILES_DIRTY="true"
fpath=(/usr/local/share/zsh-completions $fpath)

# Path to your oh-my-zsh configuration.
source "$ZSH/oh-my-zsh.sh"
source $BREW_PATH/share/zsh-syntax-highlighting/zsh-syntax-highlighting.zsh

# User configuration

export C_INCLUDE_PATH=/usr/local/include
# export MANPATH="/:$MANPATH"

export PATH="/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"
export PATH=$HOME/.local/bin:$PATH
export PATH="/opt/homebrew/bin:$PATH"
export PATH="/opt/homebrew/sbin:$PATH"

# Android
export PATH=$HOME/Library/Android/sdk/platform-tools:$PATH
export PATH=$HOME/Library/Android/sdk/tools:$PATH

# Ruby
export GEM_HOME=$HOME/.gem #is this even needed?
#export PATH=$GEM_HOME/bin:$PATH
#export PATH=$HOME/.gem/ruby/2.0.0/bin:$PATH

# export PATH=$PATH:$(yarn global bin)

# Python
export PYTHONPATH=./pip-components:$PYTHONPATH

# Shared
export PATH=$HOME/.bin-remote:$PATH

# Go
mkdir -p $HOME/.go
export GOPATH=$HOME/.go
export PATH=$PATH:$GOPATH/bin

# Node
mkdir -p $HOME/.nvm
export NVM_DIR=~/.nvm
if [ -e "$(brew --prefix nvm)/nvm.sh" ]; then
  source $(brew --prefix nvm)/nvm.sh
fi

# postgres
#
# the formula libpq conflicts with the package postgres.
# libpq only has client tools (psql, pq_dump, etc) while postgres has
# everything
if ! command -v psql &> /dev/null ; then
    export PATH="/opt/homebrew/opt/libpq/bin:$PATH"
fi

# google cloud
if [ -d "$HOME/google-cloud-sdk" ]; then
    export PATH="$HOME/google-cloud-sdk/bin:$PATH"
fi

alias emacs='\emacs -nw'
alias cd..='cd ..'
alias lss='less'
alias grep='grep --color'
alias ls='ls -hF'
alias ll='ls -l'
alias lla='ls -la'
alias la='ls -a'
alias rm='rm -i'
alias reload=". $HOME/.zshrc"
alias extract-pkg="pkgutil --expand-full " # usage extract-pkg [pkg] [out_dir]

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

    rmdir trash
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
    if [ -e "yarn.lock" ]; then
        yarn "$@"
    elif [ -e "pnpm-lock.yaml" ]; then
        pnpm "$@"
    elif [ -e "bun.lockb" ]; then
        bun run "$@"
    elif [ -e "package-lock.json" ]; then
        npm run "$@"
    elif [ -e "go.mod" ]; then
        task "$@"
    elif [ -e "manage.py" ]; then
        python manage.py "$@"
    else
        echo "Nothing to run"
        return 1
    fi
}

function install {
    if [ -e "yarn.lock" ]; then
        yarn add "$@"
    elif [ -e "pnpm-lock.yaml" ]; then
        pnpm install "$@"
    elif [ -e "bun.lockb" ]; then
        bun install "$@"
    elif [ -e "package-lock.json" ]; then
        npm install "$@"
    elif [ -e "go.mod" ]; then
        go get "$@"
    elif [ -d ".venv" ]; then
        pip install "$@"
        pip freeze > requirements.txt
    else
        echo "Nothing to run"
        return 1
    fi
}

function lint {
    if is-go-repo; then
        golangci-lint run ./... "$@"
    else
        run lint
    fi
}

function cl() {
    if [ -z "$1" ]; then
        echo "cl <repo-name>"
    fi

    local user="${GH_CLONE_USER_NAME:-Nivl}"
    git cl "$user/$1"
    cd "$1"
}
