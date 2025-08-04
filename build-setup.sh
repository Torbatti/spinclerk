#!/bin/bash
BASEDIR="$PWD"
FILE_NAME="go1.24.5.linux-amd64.tar.gz"

mkdir -p tool
cd tool
wget https://go.dev/dl/$FILE_NAME
tar -xf $FILE_NAME
mv $FILE_NAME/go $BASEDIR/tool/go
exit 0