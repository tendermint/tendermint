#! /bin/bash

go get -u $TMREPO/cmd/tendermint
cd $GOPATH/src/$TMREPO
git fetch -a origin
git reset --hard $TMHEAD
make
tendermint node
