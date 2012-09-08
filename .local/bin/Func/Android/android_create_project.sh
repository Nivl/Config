#!/bin/bash
## Comment:		Helper to create a new android project.
##
## Project:
## Started by		Melvin Laplanche <melvin.laplanche+dev@gmail.com>
## On			06/11/2011, 05:35 PM
## Last updated by	Melvin Laplanche <melvin.laplanche+dev@gmail.com>
## On			06/27/2011, 06:51 PM

function android_create_project() {
    if [ $# != 3 ] || [ -z $1 ] || [ -z $2 ] || [ -z $3 ]; then
	echo "usage: android_new_project projetName target dir"
	return 1
    fi

    local name=$1
    local lowerName=${name,,}
    local target=$2
    local path=$3
    android create project \
	--package com.nivl.$lowerName \
	--activity $name \
	--target $target \
	--path $path/$1
    return 0
}

