#!/bin/bash
## Comment:		Helper to update some emacs major-mode.
##
## Project:
## Started by		Melvin Laplanche <melvin.laplanche+dev@gmail.com>
## On			06/27/2011, 06:42 PM
## Last updated by	Melvin Laplanche <melvin.laplanche+dev@gmail.com>
## On			July 17 2011 at 03:01 PM

# Usage: update_emacs [mode_name]
function update_emacs() {
    pwd_save=`pwd`
    repo_path=/tmp/emacs-save
    autoload_path=~/.emacs.d/autoloadable
    snippets_path=~/.emacs.d/snippets

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
    array=( "pkgbuild" "android" "django" "egg" "yaml" "cmake"
	"python" "html5" "yasnippet" "snippets" "nxhtml" "markdown")

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
    __update_emacs_cmake

     # Put the modes which need to be copied above this comment

    put_info -n "Updating all retrieved mode... "
    cp $repo_path/*/*.el $autoload_path/
    __update_emacs_check_return $? "fail" "done"

    __update_emacs_pkgbuild
    __update_emacs_android
    __update_emacs_django
    __update_emacs_markdown
    __update_emacs_egg
    __update_emacs_yaml
    __update_emacs_python
    __update_emacs_html5
    __update_emacs_yasnippet
    __update_emacs_nxhtml

    put_error "The following modes are not auto-updated: "
    echo -e "pov-mode\nredo\nredo+\nphp-mode\nctypes"
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
    put_info "Updating $1"
    cd $autoload_path/$1
    git pull
    __update_emacs_check_return $? "fail" "done"
    cd "/tmp"
}

function __update_emacs_pkgbuild() {
    __update_emacs_from_git "pkgbuild-mode"
}

function __update_emacs_android() {
    __update_emacs_from_git "android-mode"
}

function __update_emacs_django() {
    __update_emacs_from_git "django-mode"
}

function __update_emacs_markdown() {
    __update_emacs_from_git "markdown-mode"
}

function __update_emacs_egg() {
    __update_emacs_from_git "django-mode"
}

function __update_emacs_cmake() {
    put_info "Retrieving cmake-mode"
    mkdir  $repo_path/cmake-mode
    wget -O  $repo_path/cmake-mode/cmake-mode.el \
	"http://www.cmake.org/CMakeDocs/cmake-mode.el"
    __update_emacs_check_return $? "fail" "done"
}

function __update_emacs_yaml() {
    __update_emacs_from_git "yaml-mode"
}

function __update_emacs_python() {
    __update_emacs_from_git "python-mode"
}

function __update_emacs_html5() {
    put_info "Updating html5-el..."
    cd $autoload_path/html5-el
    git pull
    make relaxng
    __update_emacs_check_return $? "fail" "done"
    cd /tmp
}

function __update_emacs_yasnippet() {
    put_info "Updating yasnippet"
    svn checkout http://yasnippet.googlecode.com/svn/trunk/ \
	 $repo_path/yasnippet
    cp  $repo_path/yasnippet/yasnippet.el $autoload_path/
    cp  $repo_path/yasnippet/dropdown-list.el $autoload_path/
    put_info "Updating yasnippet done"
}

function __update_emacs_snippets() {
    put_info "Updating snippets"
    cd $snippets_path
    rm -Rf *
    svn checkout http://yasnippet.googlecode.com/svn/trunk/ \
	$repo_path/yasnippet
    cp -Rf $repo_path/yasnippet/snippets/* .
    put_info "Updating snippets done"
    cd /tmp
}

function __update_emacs_nxhtml() {
    put_info "Updating nxhtml"
    cd $autoload_path/nxhtml
    bzr merge
    __update_emacs_check_return $? "fail" "done"
    cd "/tmp"
}
