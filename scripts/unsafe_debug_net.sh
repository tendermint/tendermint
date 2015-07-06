#!/bin/sh

debora run -- bash -c "cd \$GOPATH/src/github.com/tendermint/tendermint; killall tendermint"
debora run -- bash -c "cd \$GOPATH/src/github.com/tendermint/tendermint; tendermint unsafe_reset_priv_validator; rm -rf ~/.tendermint/data"
debora run -- bash -c "cd \$GOPATH/src/github.com/tendermint/tendermint; git pull origin develop; make"
debora run --bg --label tendermint -- bash -c "cd \$GOPATH/src/github.com/tendermint/tendermint; tendermint node"
echo "sleeping for a minute"
sleep 60
debora download tendermint "logs/async$1"
debora run -- bash -c "cd \$GOPATH/src/github.com/tendermint/tendermint; killall tendermint"
