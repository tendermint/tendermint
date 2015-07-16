#!/bin/bash
set -euo pipefail
IFS=$'\n\t'

printf "Stopping group $1...\n"
sleep 3

debora --group "$1" run -- bash -c "cd \$GOPATH/src/github.com/tendermint/tendermint; killall tendermint; killall logjack"
printf "Done\n"
