#!/bin/bash
## Comment:		Source all sh file
##
## Project:
## Started by		Melvin Laplanche <melvin.laplanche+dev@gmail.com>
## On			June 29 2011 at 10:24 PM
## Last updated by	Melvin Laplanche <melvin.laplanche+dev@gmail.com>
## On			June 29 2011 at 10:38 PM

source_functions() {
    if [ -d $1 ]; then
	for f in $1/*; do
	    if [ -d $f ]; then
		source_functions $f
	    else
		if [ ${f/*./} == "sh" ]; then
		    . $f
		fi
	    fi
	done
	return 0
    fi
    return 1
}
