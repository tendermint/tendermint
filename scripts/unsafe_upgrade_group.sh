#!/bin/bash
set -euo pipefail
IFS=$'\n\t'

printf "Upgrading group $1...\n"
sleep 3

debora --group "$1" run -- bash -c "cd \$GOPATH/src/github.com/tendermint/tendermint; git pull origin develop; make"
printf "Done\n"
