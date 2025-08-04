#!/bin/bash
BASEDIR="$PWD"

export GOROOT=$BASEDIR/tool/go
export GOPATH=$GOROOT/bin
export GO=$GOPATH/go

# $GO tool dist list
export GOOS=linux
export GOARCH=amd64

# --- Unpack Arguments --------------------------------------------------------
for arg in "$@"; do declare $arg='1'; done

if [ -v build ]; then build=1 && echo "[ build enabled ]"; fi
if [ -v debug ]; then debug=1 && echo "[ debug enabled ]"; fi
if [ -v run ]; then run=1 && echo "[ run enabled ]"; fi
if [ -v tidy ]; then tidy=1 && echo "[ tidy enabled ]"; fi

if [ -v proxy ]; then proxy=1 && echo "[ proxy enabled ]"; fi
if [ -v inside ]; then inside=1 && echo "[ inside enabled ]"; fi
if [ -v outside ]; then outside=1 && echo "[ outside enabled ]"; fi

# --- Prep Directories --------------------------------------------------------
mkdir -p build

# --- Pre Build/Run/Debug Step ------------------------------------------------
if [[ -z $proxy ]] && [[ -z $inside ]] && [[ -z $outside ]]
then
    echo "[ NOTE: you did not specified a project,SELECT ALL is default! ]" 
    proxy=1;
    inside=1;
    outside=1;
fi

if [[ $run -eq 1 ]] && [[ $debug -eq 1 ]]
then 
    echo "[ ERROR: you CAN NOT run and debug at the same time ! ]"
    exit 1
fi

if [[ $run -eq 1 ]] || [[ $debug -eq 1 ]]
then

    if [[ $proxy -eq 1 ]] && [[ $inside -eq 1 ]]
    then
        rd_multi_file=1;
    fi
    if [[ $proxy -eq 1 ]] && [[ $outside -eq 1 ]]
    then
        rd_multi_file=1;
    fi
    if [[ $inside -eq 1 ]] && [[ $outside -eq 1 ]]
    then
        rd_multi_file=1;
    fi

    if [[ $rd_multi_file -eq 1 ]] 
    then 
        echo "[ ERROR: you CAN NOT run/debug multiple projects at the same time ! ]" 
        exit 1
    fi
fi
# --- Tidy ; Build ; Run ; Debug ----------------------------------------------
if [[ $tidy -eq 1 ]]
then 
    cd proxy
    if [[ $proxy -eq 1 ]]; then $GO mod tidy && echo "[ tidy proxy project ]";fi
    cd ..

    cd inside
    if [[ $inside -eq 1 ]]; then $GO mod tidy && echo "[ tidy inside project ]";fi
    cd ..

    cd outside
    if [[ $outside -eq 1 ]]; then $GO mod tidy && echo "[ tidy outside project ]";fi
    cd ..
fi

if [[ $build -eq 1 ]]
then 
    cd proxy
    if [[ $proxy -eq 1 ]]; then $GO build -o ../build/proxy main.go && echo "[ built proxy bin ]";fi
    cd ..

    cd inside
    if [[ $inside -eq 1 ]]; then $GO build -o ../build/inside main.go && echo "[ built inside bin ]";fi
    cd ..

    cd outside
    if [[ $outside -eq 1 ]]; then $GO build -o ../build/outside main.go && echo "[ built outside bin ]";fi
    cd ..
fi

cd build
if [[ $run -eq 1 ]]
then 
    if [[ $proxy -eq 1 ]]; then ./proxy ;fi
    if [[ $inside -eq 1 ]]; then ./inside ;fi
    if [[ $outside -eq 1 ]]; then ./outside ;fi
fi

if [[ $debug -eq 1 ]]
then
    echo "[ debugger starting ]"
    echo "[ https://go.dev/doc/gdb ]"
    if [[ $proxy -eq 1 ]]; then gdb --silent ./proxy ;fi
    if [[ $inside -eq 1 ]]; then gdb --silent ./inside ;fi
    if [[ $outside -eq 1 ]]; then gdb --silent ./outside ;fi
fi
cd ..

# --- Exit --------------------------------------------------------------------
exit 0
