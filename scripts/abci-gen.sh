#!/usr/bin/env bash
set -euo pipefail

# By default, this script runs against the latest commit to the master branch
# in the Tendermint spec repository. To use this script with a different version
# of the spec repository, run it with the $VERS environment variable set to the
# desired branch name or commit hash from the spec repo.


MODNAME="$(go list -m)"
find ./proto/tendermint -name '*.proto' -not -path "./proto/tendermint/abci/types.proto" \
	-exec sh ./scripts/protopackage.sh {} "$MODNAME" ';'

# For historical compatibility, the abci file needs to get a slightly different import name
# so that it can be moved into the ./abci/types directory.
sh ./scripts/protopackage.sh ./proto/tendermint/abci/types.proto $MODNAME "abci/types"

make proto-gen

mv ./proto/tendermint/abci/types.pb.go ./abci/types

echo "proto files have been compiled"

echo "removing copied files"

find proto/tendermint/ -name '*.proto' -not -path "proto/tendermint/abci/types.proto" -not -path "proto/tendermint/types/types.proto" \
	| xargs -I {} git checkout {}

find proto/tendermint/ -name '*.pb.go' -not -path "proto/tendermint/abci/types.pb.go" -not -path "proto/tendermint/types/types.pb.go" \
	| xargs -I {} git checkout {}
