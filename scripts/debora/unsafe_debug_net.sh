#!/bin/bash
set -euo pipefail
IFS=$'\n\t'

debora run -- bash -c "cd \$GOPATH/src/github.com/tendermint/tendermint; killall tendermint"
debora run -- bash -c "cd \$GOPATH/src/github.com/tendermint/tendermint; tendermint unsafe_reset_priv_validator; rm -rf ~/.tendermint/data"
debora run -- bash -c "cd \$GOPATH/src/github.com/tendermint/tendermint; git pull origin develop; make"
debora run -- bash -c "cd \$GOPATH/src/github.com/tendermint/tendermint; mkdir -p ~/.tendermint/logs"
debora run --bg --label tendermint -- bash -c "cd \$GOPATH/src/github.com/tendermint/tendermint; tendermint node 2>&1 | stdinwriter -outpath ~/.tendermint/logs/tendermint.log"
printf "\n\nSleeping for a minute\n"
sleep 60
debora download tendermint "logs/async$1"
debora run -- bash -c "cd \$GOPATH/src/github.com/tendermint/tendermint; killall tendermint"
