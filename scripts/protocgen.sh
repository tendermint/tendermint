#!/usr/bin/env bash

set -eo pipefail

buf generate --path proto/tendermint

mv ./proto/tendermint/abci/types.pb.go ./abci/types
