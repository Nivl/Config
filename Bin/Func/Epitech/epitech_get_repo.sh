#!/bin/bash
## Comment:		Helper to checkout a svn/git/hg repo.
##
## Project:
## Started by		Melvin Laplanche <melvin.laplanche+dev@gmail.com>
## On			January 05 2012 at 03:44 PM
## Last updated by	melvin laplanche <melvin.laplanche+dev@gmail.com>
## On			February 02 2012 at 04:56 PM

function epitech_get_repo() {
    if [ $# != 2 ] || [ -z $1 ] || [ -z $2 ]; then
	echo "usage: epitech_get_repo type name"
	return 1
    fi

    local type=${1,,}
    local repo=$2

    if [ "$type" = "git" ]; then
	git clone kscm@koala-rendus.epitech.net:$repo
    elif [ "$type" = "hg" ]; then
	hg clone ssh://kscm@koala-rendus.epitech.net/$repo
    elif [ "$type" = "svn" ]; then
	svn checkout svn+ssh://kscm@koala-rendus.epitech.net/$repo
    else
	put_error "$type are not supported. The supported DRCS are svn, git, and hg"
	return 1
    fi

    return 0
}

