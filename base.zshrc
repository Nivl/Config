#!/bin/env zsh

zstyle ':completion:*' special-dirs true
zstyle ':omz:update' mode disabled # Updates should be done manually

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
source $HOMEBREW_PREFIX/share/zsh-syntax-highlighting/zsh-syntax-highlighting.zsh

export C_INCLUDE_PATH=/usr/local/include
# export MANPATH="/:$MANPATH"

export PATH="/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"
export PATH=$HOME/.local/bin:$PATH
export PATH="$HOMEBREW_PREFIX/bin:$PATH"
export PATH="$HOMEBREW_PREFIX/sbin:$PATH"

_melvin_check_update() {
    local current_time=$(date +%s)
    local last_check=0

    _melvin_config_dir="$HOME/.melvin/config"
    _melvin_update_check_file="$_melvin_config_dir/.last_update_check"
    _melvin_two_weeks=$((14 * 24 * 60 * 60))

    if [[ -f "$_melvin_update_check_file" ]]; then
        last_check=$(cat "$_melvin_update_check_file")
    fi

    if (( current_time - last_check >= _melvin_two_weeks )); then
        # part of this logic is also in install.sh
        #
        # It's a bit hacky to add the time BEFORE the update, but it's
        # to handle the case where we open multiple shell at the same time.
        # it may not work perfectly well, but that better than nothing.
        echo "$current_time" > "$_melvin_update_check_file"

        while true; do
            echo -n "Would you like to update your config? (Y/N): "
            read answer
            case "$answer" in
                "y"|"Y" )
                    bash "$_melvin_config_dir/install.sh"
                    break
                    ;;
                "n"|"N" )
                    break
                    ;;
                * )
                    echo "Please answer Y or N."
                    ;;
            esac
        done
    fi
}

_melvin_check_update

function update-cfg {
    bash "$_melvin_config_dir/install.sh"
}

source $HOME/.melvin/config/config.zshrc
