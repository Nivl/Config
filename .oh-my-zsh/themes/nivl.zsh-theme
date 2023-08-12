#!/bin/bash

# Nivl's Theme based on Agnoster's Theme (https://gist.github.com/3712874)
# also inspired from Spaceship (https://github.com/denysdovhan/spaceship-prompt)

#
# Utils
#

displaytime() {
  local T=$1
  local D=$((T/60/60/24))
  local H=$((T/60/60%24))
  local M=$((T/60%60))
  local S=$((T%60))
  [[ $D > 0 ]] && printf '%dd ' $D
  [[ $H > 0 ]] && printf '%dh ' $H
  [[ $M > 0 ]] && printf '%dm ' $M
  printf '%ds' $S
}

# Begin a segment
# Takes 4 arguments
# - background
# - foreground
# - The text to display
# - Whether or not we want to display a space before the text
CURRENT_BG='NONE'
prompt_segment() {
  local bg fg
  [[ -n $1 ]] && bg="%K{$1}" || bg="%k"
  [[ -n $2 ]] && fg="%F{$2}" || fg="%f"
  if [[ $CURRENT_BG != 'NONE' && $1 != $CURRENT_BG && $1 != 'NONE' ]]; then
    echo -n "1 %{$bg%F{$CURRENT_BG}%}%{$fg%}"
  else
    echo -n "%{$bg%}%{$fg%}"
  fi

  if [[ $4 != 'NO_SPACE' ]]; then
    echo -n " "
  fi

  CURRENT_BG=$1
  [[ -n $3 ]] && echo -n $3
}

#
# Hooks
#
autoload -Uz add-zsh-hook
add-zsh-hook preexec exec_time_preexec_hook
add-zsh-hook precmd exec_time_precmd_hook

# Execution time start
exec_time_preexec_hook() {
  PROMPT_EXEC_TIME_START=${EPOCHREALTIME}
}

# Execution time end
exec_time_precmd_hook() {
  [[ -n $PROMPT_EXEC_TIME_DURATION ]] && unset PROMPT_EXEC_TIME_DURATION
  [[ -z $PROMPT_EXEC_TIME_START ]] && return
  local PROMPT_EXEC_TIME_STOP=${EPOCHREALTIME}
  PROMPT_EXEC_TIME_DURATION=$(echo $(( $PROMPT_EXEC_TIME_STOP - $PROMPT_EXEC_TIME_START ))  | cut -c1-5)
  unset PROMPT_EXEC_TIME_START
}

#
# Prompt
#

# End the prompt, closing any open segments
prompt_end() {
  if [[ -n $CURRENT_BG ]]; then
    echo -n " %{%k%F{$CURRENT_BG}%}"
  else
    echo -n "%{%k%}"
  fi
  echo -n "%{%f%}"
  CURRENT_BG=''
}

### Prompt components
# Each component will draw itself, and hide itself if no information needs to be shown

# Context: user@hostname (who am I and where am I)
prompt_context() {
  if [ -n "$SCRIPT" ]; then
    prompt_segment NONE red "●REC " NO_SPACE
  fi

  if [[ "$USER" != "$DEFAULT_USER" || -n "$SSH_CLIENT" ]]; then
    prompt_segment NONE default "%(!.%{%F{yellow}%}.)$USER@%m"
  fi
}

# Git: branch/detached head, dirty status
# That's the slow method. Doing some async work could be cool
prompt_git() {
  (( $+commands[git] )) || return
  local ref dirty mode repo_path
  repo_path=$(git rev-parse --git-dir 2>/dev/null)

  if $(git rev-parse --is-inside-work-tree >/dev/null 2>&1); then
    dirty=$(parse_git_dirty)
    ref=$(git symbolic-ref HEAD 2> /dev/null) || ref="➦ $(git rev-parse --short HEAD 2> /dev/null)"

    if [[ -n $dirty ]]; then
      prompt_segment NONE yellow
    else
      prompt_segment NONE green
    fi

    if [[ -e "${repo_path}/BISECT_LOG" ]]; then
      mode=" <B>"
    elif [[ -e "${repo_path}/MERGE_HEAD" ]]; then
      mode=" >M<"
    elif [[ -e "${repo_path}/CHERRY_PICK_HEAD" ]]; then
      mode=" >CP<"
    elif [[ -e "${repo_path}/rebase" || -e "${repo_path}/rebase-apply" || -e "${repo_path}/rebase-merge" || -e "${repo_path}/../.dotest" ]]; then
      mode=" >R>"
    fi

    setopt promptsubst
    autoload -Uz vcs_info

    zstyle ':vcs_info:*' enable git
    zstyle ':vcs_info:*' get-revision true
    zstyle ':vcs_info:*' check-for-changes true
    zstyle ':vcs_info:*' stagedstr '+'
    zstyle ':vcs_info:*' unstagedstr '!'
    zstyle ':vcs_info:*' formats ' %u%c'
    zstyle ':vcs_info:*' actionformats ' %u%c'
    vcs_info

    local untrackedstr=""
    [[ $(git status --porcelain | grep '??') ]] && untrackedstr="?"
    [[ -z "${vcs_info_msg_0_%% }" &&  ! -z "$untrackedstr" ]] && untrackedstr=" ?"

    echo -n "${ref/refs\/heads\//"ᚠ" }${vcs_info_msg_0_%% }${untrackedstr}${mode}"
  fi
}

git_toplevel() {
  local repo_root=$(git rev-parse --show-toplevel)
  if [[ $repo_root = '' ]]; then
    # We are in a bare repo. Use .git dir as root
    repo_root=$(git rev-parse --git-dir)
    if [[ $repo_root = '.' ]]; then
      $repo_root=$(psw)
    fi
  fi
  echo -n $repo_root
}

# sysinfo: info about the system: battery, time, ...
prompt_sysinfo() {
  local battery=$(pmset -g batt | grep -oE '[0-9]{1,3}%' | tr -d '%[,;]')

  if [ -n "$battery" ]; then
    if [ $battery -le 25 ]; then
      prompt_segment NONE red "$battery%% " NO_SPACE
    elif [ $battery -le 50 ]; then
      prompt_segment NONE yellow "$battery%% " NO_SPACE
    fi
  fi

  prompt_segment NONE magenta "%*" NO_SPACE
}

# Dir: current working directory
prompt_dir() {
  if $(git rev-parse --is-inside-work-tree >/dev/null 2>&1); then
    local repo_root=$(git_toplevel)
    local root_folder=$(basename $repo_root)
    local path_in_repo=$(pwd | sed "s/^$(echo "$repo_root" | sed 's:/:\\/:g;s/\$/\\$/g')//;s:^/::;s:/$::;")
    if [[ $path_in_repo != '' ]]; then
      path_in_repo="/$path_in_repo"
    fi

    prompt_segment NONE blue "$root_folder$path_in_repo"
  else
    prompt_segment NONE blue '%~'
  fi
}

# Status:
# - was there an error
# - am I root
# - am i dumb?
prompt_status() {
  local symbols last_command random_seed
  symbols=()
  # fc doc: http://zsh.sourceforge.net/Doc/Release/Shell-Builtin-Commands.html
  # -L limits to the history of th current session
  # -l is to list the data (otherwishe it opens $EDITOR)
  # -1 is to get 1 from the end
  # the output will contain the history number and some some spacings
  # that's fine for our usage
  #   ex: "  339  ls README.md"
  last_command=$(fc -l -L -1)

  [[ $RETVAL -ne 0 ]] && symbols+="%{%F{red}%}oops"
  [[ $UID -eq 0 ]] && symbols+="%{%F{yellow}%} I am groot"
  [[ $last_command ==  *"sudo "* ]] && symbols+="%{%F{yellow}%}Previous ran as root"
  symbols+="%{%F{magenta}%}ran in ${PROMPT_EXEC_TIME_DURATION}s"

  [[ -n "$symbols" ]] && prompt_segment NONE default "$symbols"
}

# shows the prompt symbol (regular ❯, error ✘, root ⚡, has BG jobs ✦)
prompt_line2() {
  local symbols
  symbols=()
  echo "" # new line
  [[ $RETVAL -eq 0 ]] && symbols+="%{%F{blue}%}❯"
  [[ $RETVAL -ne 0 ]] && symbols+="%{%F{red}%}✘"
  [[ $UID -eq 0 ]] && symbols+="%{%F{yellow}%}⚡"
  [[ $(jobs -l | wc -l) -gt 0 ]] && symbols+="%{%F{cyan}%}✦"

  [[ -n "$symbols" ]] && prompt_segment NONE default "$symbols" NO_SPACE
}

## Main prompt
build_prompt() {
  RETVAL=$?

  prompt_context
  prompt_sysinfo
  prompt_dir
  prompt_git
  prompt_status
  # prompt_line2 # Not needed for Warp, add for Iterm 2
  prompt_end
}

PROMPT='%{%f%b%k%}$(build_prompt)'
