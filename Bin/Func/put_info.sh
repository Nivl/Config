#!/bin/bash
## Comment:		Display an information
##
## Project:
## Started by		Melvin Laplanche <melvin.laplanche+dev@gmail.com>
## On			June 29 2011 at 06:53 PM
## Last updated by	Melvin Laplanche <melvin.laplanche+dev@gmail.com>
## On			June 30 2011 at 07:05 PM

function put_info() {
    echo -en "\033[01;34m"
    echo $@
    echo -en "\033[00m"
    return 0
}
