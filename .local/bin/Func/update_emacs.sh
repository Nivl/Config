#!/bin/bash
## Comment:		Helper to update some emacs major-mode.
##
## Project:
## Started by		Melvin Laplanche <melvin.laplanche+dev@gmail.com>
## On			06/27/2011, 06:42 PM
## Last updated by	melvin laplanche <melvin.laplanche+dev@gmail.com>
## On			April 10 2012 at 12:05 AM

# Usage: update_emacs [mode_name]
function update_emacs() {
    pwd_save=`pwd`
    repo_path=/tmp/emacs-save/
    conf_path=~/.emacs.d/
    autoload_path=~/.emacs.d/autoloadable
    snippets_path=~/.emacs.d/snippets
    git_root=~/

    mkdir -p $repo_path
    if [ $? -ne 0 ]; then
	put_error "$repo_path not writable"
	return 0;
    fi

    rm -Rf $repo_path
    cd "/tmp"

    if [ -z $1 ]; then
	__update_emacs_all
    else
	__update_emacs_one $1
    fi

    rm -Rf $repo_path
    cd $pwd_save
}

function __update_emacs_one() {
    local found=0
    array=( "pkgbuild" "egg" "snippets" "nxhtml")

    for (( i=0; i<${#array[@]}; i++ )); do
	if [ "$1" == "${array[$i]}" ]; then
	    local cmd="__update_emacs_${array[$i]}"
	    eval $cmd
	    return 0
	fi
    done
    if [ $found -eq 0 ]; then
	put_error "$1 is not a valid mode"
    fi
    return 1
}

function __update_emacs_all() {

    cd $git_root
    put_info "Updating pkgbuild and egg"
    git submodule update
    __update_emacs_check_return $? "fail" "done"
    cd "/tmp"

    __update_emacs_snippets
    __update_emacs_nxhtml

    put_error "The following modes are not auto-updated: "
    echo -e "redo\n"
}

# Usage : $? message_if_fail [message_if_ok]
function __update_emacs_check_return() {
    if [ $# -ne 2 ] && [ $# -ne 3 ]; then
	put_error "usage: $? message_if_fail [message_if_ok]"
	return 1
    fi
    if [ $1 -eq 0 ]; then
	if [ ! -z $3 ]; then
	    put_info $3
	fi
    else
	put_error $2
    fi
    return 0
}

# Usage : mode-name
function __update_emacs_from_git() {
    put_info -n "Updating $1... "
    cd ~
    git submodule update $autoload_path/$1
    __update_emacs_check_return $? "fail" "done"
    cd "/tmp"
}

function __update_emacs_pkgbuild() {
    __update_emacs_from_git "pkgbuild-mode"
}

function __update_emacs_egg() {
    __update_emacs_from_git "egg-mode"
}

function __update_emacs_snippets() {
    put_info "Updating snippets"
    cd $snippets_path
    rm -Rf *
    git clone https://github.com/capitaomorte/yasnippet.git \
	$repo_path/yasnippet
    cp -Rf $repo_path/yasnippet/snippets/* .
    put_info "Updating snippets done"
    cd /tmp
}

function __update_emacs_nxhtml() {
    put_info "Updating nxhtml"
    cd $autoload_path/nxhtml
    bzr merge --force
    __update_emacs_check_return $? "fail" "done"
    cd "/tmp"
}
