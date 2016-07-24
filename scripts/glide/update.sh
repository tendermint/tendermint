#! /bin/bash
set -euo pipefail
IFS=$'\n\t'

# script to update the given dependency in the glide.lock file with the checked out branch on the local host

GLIDE=$1
LIB=$2

OLD_COMMIT=`bash scripts/glide/parse.sh $GLIDE $LIB`

PWD=`pwd`
cd $GOPATH/src/github.com/tendermint/$LIB

NEW_COMMIT=$(git rev-parse HEAD)

cd $PWD
sed -i "s/$OLD_COMMIT/$NEW_COMMIT/g" $GLIDE
