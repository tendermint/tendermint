#!/usr/bin/env bash
set -euo pipefail

# By default, this script runs against the latest commit to the master branch
# in the Tendermint spec repository. To use this script with a different version
# of the spec repository, run it with the $VERS environment variable set to the
# desired commit in the spec repo.

: ${VERS:=master}
URL_PATH=tarball/
if [[ VERS -ne master ]]; then
    URL_PATH=tarball/refs/tags/v
fi

echo "fetching proto files"

# Get shortened ref of commit
REF=$(curl -H "Accept: application/vnd.github.v3.sha" -qL \
  "https://api.github.com/repos/tendermint/spec/commits/${VERS}" \
  | cut -c -7)
curl -qL "https://api.github.com/repos/tendermint/spec/${URL_PATH}${REF}" | tar -xzf - tendermint-spec-"$REF"/

cp -r ./tendermint-spec-"$REF"/proto/tendermint/* ./proto/tendermint
cp -r ./tendermint-spec-"$REF"/third_party/** ./third_party

MODNAME=$(go list -m)
find ./proto/tendermint -name *.proto -exec sh ./scripts/protopackage.sh {} $MODNAME ';'

buf generate --path proto/tendermint --template ./tendermint-spec-"$REF"/proto/buf.gen.yaml --config ./tendermint-spec-"$REF"/proto/buf.yaml

mv ./proto/tendermint/abci/types.pb.go ./abci/types

echo "proto files have been compiled"

echo "removing copied files"

rm -rf ./proto/tendermint/abci
rm -rf ./proto/tendermint/blocksync/types.proto
rm -rf ./proto/tendermint/consensus/types.proto
rm -rf ./proto/tendermint/libs/bits/types.proto
rm -rf ./proto/tendermint/mempool/types.proto
rm -rf ./proto/tendermint/p2p/types.proto
rm -rf ./proto/tendermint/p2p/conn.proto
rm -rf ./proto/tendermint/p2p/pex.proto
rm -rf ./proto/tendermint/statesync/types.proto
rm -rf ./proto/tendermint/types/block.proto
rm -rf ./proto/tendermint/types/evidence.proto
rm -rf ./proto/tendermint/types/params.proto
rm -rf ./proto/tendermint/types/types.proto
rm -rf ./proto/tendermint/types/validator.proto
rm -rf ./proto/tendermint/version/types.proto
rm -rf ./thirdparty/proto/gogoproto/gogo.proto

rm -rf ./tendermint-spec-"$REF"
