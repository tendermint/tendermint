#! /bin/bash

cd "$GOPATH/src/github.com/tendermint/tendermint"

bash ./test/persist/test_failure_indices.sh
