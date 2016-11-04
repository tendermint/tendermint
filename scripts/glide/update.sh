#! /bin/bash
set -euo pipefail
IFS=$'\n\t'

# script to update the given dependency in the glide.lock file with the checked out branch on the local host

LIB=$1

TMCORE=$GOPATH/src/github.com/tendermint/tendermint
set +u
if [[ "$GLIDE" == "" ]]; then
	GLIDE=$TMCORE/glide.lock
fi
set -u

OLD_COMMIT=`bash $TMCORE/scripts/glide/parse.sh $LIB`

PWD=`pwd`
cd $GOPATH/src/github.com/tendermint/$LIB

NEW_COMMIT=$(git rev-parse HEAD)

cd $PWD

uname -a | grep Linux > /dev/null
if [[ "$?" == 0 ]]; then
	# linux
	sed -i "s/$OLD_COMMIT/$NEW_COMMIT/g" $GLIDE
else 
	# mac 
	sed -i "" "s/$OLD_COMMIT/$NEW_COMMIT/g" $GLIDE
fi
