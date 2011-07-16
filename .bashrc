#!/bin/bash
## Comment:
##
## Project:
## Started by		Melvin Laplanche <melvin.laplanche+dev@gmail.com>
## On			June 11 2011 at 17:21 PM
## Last updated by	Melvin Laplanche <melvin.laplanche+dev@gmail.com>
## On			July 17 2011 at 01:59 AM

. ~/Bin/Func/source.sh
source_functions ~/Bin/Func

eval "`dircolors -b`"
export PATH="/usr/lib/colorgcc/bin:$PATH"
export PATH="/opt/android-sdk/platform-tools:$PATH"
export PATH="~/Bin:$PATH"

export LESS_TERMCAP_mb=$'\E[01;31m'
export LESS_TERMCAP_md=$'\E[01;31m'
export LESS_TERMCAP_me=$'\E[0m'
export LESS_TERMCAP_so=$'\E[01;44;33m'
export LESS_TERMCAP_se=$'\E[0m'
export LESS_TERMCAP_us=$'\E[01;32m'
export LESS_TERMCAP_ue=$'\E[0m'

export EDITOR='emacs -nw'
export PAGER='less'
export PS1="\[\033[01;32m\][\u@\h \w]\$\[\033[00m\] "
# For Root
# export PS1="\[\033[01;31m\]\h:\w#\[\033[00m\] "

# To make .XCompose working in GTK apps
GTK_IM_MODULE=xim
export GTK_IM_MODULE

# To use pymacs since python is referring to python3
PYMACS_PYTHON=python2

alias mman='man -M ~/Src/Epitech/man/'
alias norme="python2 ~/Bin/python2/norme"
alias repo="python2 ~/Bin/python2/repo"
alias django='python2 manage.py '

alias cd..='cd ..'
alias lss='less'
alias grep='grep --color'
alias ls='ls -hF --color=auto'
alias ll='ls -l'
alias lla='ls -la'
alias la='ls -a'
alias rm='rm -i'
alias emacs='\emacs -nw'
alias ne='\emacs -nw'
