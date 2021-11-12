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

MODNAME="$(go list -m)"
find ./proto/tendermint -name '*.proto' -not -path "./proto/tendermint/abci/types.proto" \
	-exec sh ./scripts/protopackage.sh {} "$MODNAME" ';'

# For historical compatibility, the abci file needs to get a slightly different import name
# so that it can be moved into the ./abci/types directory.
sh ./scripts/protopackage.sh ./proto/tendermint/abci/types.proto $MODNAME "abci/types"

buf generate --path proto/tendermint --template ./tendermint-spec-"$REF"/proto/buf.gen.yaml --config ./tendermint-spec-"$REF"/proto/buf.yaml

mv ./proto/tendermint/abci/types.pb.go ./abci/types

echo "proto files have been compiled"

echo "removing copied files"

find ./tendermint-spec-"$REF"/proto/tendermint/ -name *.proto \
	| sed "s/\.\/tendermint-spec-$REF\/\(.*\)/\1/g" \
	| xargs -I {} rm {}

rm -rf ./tendermint-spec-"$REF"
