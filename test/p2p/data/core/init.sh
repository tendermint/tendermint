#! /bin/bash
# This is a sample bash script for tendermint core
# Edit this script before "mintnet start" to change
# the core blockchain engine.

TMREPO="github.com/tendermint/tendermint"
BRANCH="master"

go get -d $TMREPO/cmd/tendermint
### DEPENDENCIES (example)
# cd $GOPATH/src/github.com/tendermint/abci
# git fetch origin $BRANCH
# git checkout $BRANCH
### DEPENDENCIES END
cd $GOPATH/src/$TMREPO
git fetch origin $BRANCH
git checkout $BRANCH
make install

tendermint node --p2p.seeds="$TMSEEDS" --moniker="$TMNAME" --proxy_app="$PROXYAPP" --rpc.unsafe
