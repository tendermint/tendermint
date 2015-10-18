#! /bin/bash

mkdir -p $GOPATH/src/$TMREPO
git clone https://$TMREPO.git .
git fetch
git reset --hard $TMHEAD
go get $TMREPO/cmd/tendermint
make
tendermint node
