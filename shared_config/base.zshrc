#!/bin/env zsh

zstyle ':completion:*' special-dirs true
zstyle ':omz:update' mode disabled # Updates should be done manually

export HOMEBREW_REQUIRE_TAP_TRUST=1

export ZSH="$HOME/.melvin/config/shared_config/.oh-my-zsh"
# ZSH_CUSTOM points OUTSIDE the OMZ submodule so user customizations
# (e.g. the nivl theme) survive `git submodule update --remote`
# bumps. Upstream OMZ's .gitignore excludes `custom/`, so anything
# placed inside $ZSH/custom would also be ignored — keeping our
# customizations as a sibling dir is the canonical OMZ pattern.
export ZSH_CUSTOM="$HOME/.melvin/config/shared_config/.oh-my-zsh-custom"
ZSH_THEME="nivl"
HIST_STAMPS="mm/dd/yyyy"
DISABLE_LS_COLORS="false"
# Skip OMZ's compaudit security check on every shell. Saves ~20-50ms.
# Re-enable temporarily if you suspect a completion file has bad perms.
ZSH_DISABLE_COMPFIX=true
plugins=(brew history extract encode64 urltools)
DEFAULT_USER=melvin
# enable command auto-correction (annoying because of "git co" being corrected as "git ci", etc.)
# ENABLE_CORRECTION="true"
# COMPLETION_WAITING_DOTS="true"
# uncomment to make VCS much faster on big repo
# DISABLE_UNTRACKED_FILES_DIRTY="true"
fpath=(/usr/local/share/zsh-completions $fpath)

# Path to your oh-my-zsh configuration.
source "$ZSH/oh-my-zsh.sh"
source $HOMEBREW_PREFIX/share/zsh-syntax-highlighting/zsh-syntax-highlighting.zsh

export C_INCLUDE_PATH=/usr/local/include
# export MANPATH="/:$MANPATH"

export PATH="/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"
export PATH=$HOME/.local/bin:$PATH
export PATH="$HOMEBREW_PREFIX/bin:$PATH"
export PATH="$HOMEBREW_PREFIX/sbin:$PATH"
# melvin-config is built into the config repo itself (see
# install.bootstrap.sh — $CONFIG_DIR/shared_config/.bin-remote).
# Prepend so it shadows any stale copy a previous install may have
# left at $HOME/.local/bin/melvin-config.
export PATH="$HOME/.melvin/config/shared_config/.bin-remote:$PATH"

# Periodic update-check hook. The stamp/prompt/exec logic lives in
# Go (`melvin-config check-update`) so there's one source of truth
# for the update flow. We guard with command -v because a brand-new
# shell on a half-bootstrapped machine may not have the binary on
# PATH yet — letting the rest of base.zshrc still load.
if command -v melvin-config >/dev/null 2>&1; then
  melvin-config check-update
fi

source $HOME/.melvin/config/shared_config/config.zshrc
