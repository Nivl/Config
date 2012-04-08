#!/bin/bash
## Comment:		Unlock config files
##
## Project:
## Started by		Melvin Laplanche <melvin.laplanche+dev@gmail.com>
## On			April 03 2011 at 01:52 PM
## Last updated by	melvin laplanche <melvin.laplanche+dev@gmail.com>
## On			April 03 2012 at 02:02 PM

function unlock() {
    if [ ${HOME} == "/root" ]; then
	put_error "You are already root!"
	return 1;
    fi
    su -c "chown laplan_m:users ${HOME}/.bashrc;
    chown laplan_m:users ${HOME}/.bash_profile;
    chown laplan_m:users ${HOME}/Bin -R"
    return 0
}
