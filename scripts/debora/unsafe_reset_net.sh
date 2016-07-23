#!/bin/bash
set -euo pipefail
IFS=$'\n\t'

debora run -- bash -c "cd \$GOPATH/src/github.com/tendermint/tendermint; killall tendermint; killall logjack"
debora run -- bash -c "cd \$GOPATH/src/github.com/tendermint/tendermint; tendermint unsafe_reset_priv_validator; rm -rf ~/.tendermint/data; rm ~/.tendermint/genesis.json; rm ~/.tendermint/logs/*"
debora run -- bash -c "cd \$GOPATH/src/github.com/tendermint/tendermint; git pull origin develop; make"
debora run -- bash -c "cd \$GOPATH/src/github.com/tendermint/tendermint; mkdir -p ~/.tendermint/logs"
debora run --bg --label tendermint -- bash -c "cd \$GOPATH/src/github.com/tendermint/tendermint; tendermint node 2>&1 | stdinwriter -outpath ~/.tendermint/logs/tendermint.log"
debora run --bg --label logjack    -- bash -c "cd \$GOPATH/src/github.com/tendermint/tendermint; logjack -chopSize='10M' -limitSize='1G' ~/.tendermint/logs/tendermint.log"
printf "Done\n"
