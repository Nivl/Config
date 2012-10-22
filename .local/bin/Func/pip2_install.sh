#!/bin/bash
## Comment:		Helper to checkout a svn/git/hg repo.
##
## Project:
## Started by		Melvin Laplanche <melvin.laplanche+dev@gmail.com>
## On			September 08 2012 at 05:45 PM
## Last updated by	Melvin Laplanche <melvin.laplanche+dev@gmail.com>
## On			September 08 2012 at 06:03 PM

function pip2() {
    local pip2_exec=`which pip2`

    if [ "$1" == "install" ]; then
	$pip2_exec $@ --user
    else
	$pip2_exec $@
    fi
}
