#!/bin/bash
set -euo pipefail
IFS=$'\n\t'

printf "Starting group $1...\n"
sleep 3

debora --group "$1" run --bg --label tendermint -- bash -c "cd \$GOPATH/src/github.com/tendermint/tendermint; tendermint node 2>&1 | stdinwriter -outpath ~/.tendermint/logs/tendermint.log"
debora --group "$1" run --bg --label logjack    -- bash -c "cd \$GOPATH/src/github.com/tendermint/tendermint; logjack -chopSize='10M' -limitSize='1G' ~/.tendermint/logs/tendermint.log"
printf "Done\n"
