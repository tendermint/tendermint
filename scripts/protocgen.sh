#!/usr/bin/env bash
set -euo pipefail

# By default, this script runs against the latest commit to the master branch
# in the Tendermint spec repository. To use this script with a different version
# of the spec repository, run it with the $VERS environment variable set to the
# desired branch name or commit hash from the spec repo.

: ${VERS:=master}

echo "fetching proto files"

# Get shortened ref of commit
REF=$(curl -H "Accept: application/vnd.github.v3.sha" -qL \
  "https://api.github.com/repos/tendermint/spec/commits/${VERS}" \
  | cut -c -7)

readonly OUTDIR="tendermint-spec-${REF}"
curl -qL "https://api.github.com/repos/tendermint/spec/tarball/${REF}" | tar -xzf - ${OUTDIR}/

cp -r ${OUTDIR}/proto/tendermint/* ./proto/tendermint
cp -r ${OUTDIR}/third_party/** ./third_party

MODNAME="$(go list -m)"
find ./proto/tendermint -name '*.proto' -not -path "./proto/tendermint/abci/types.proto" \
	-exec sh ./scripts/protopackage.sh {} "$MODNAME" ';'

# For historical compatibility, the abci file needs to get a slightly different import name
# so that it can be moved into the ./abci/types directory.
sh ./scripts/protopackage.sh ./proto/tendermint/abci/types.proto $MODNAME "abci/types"

buf generate --path proto/tendermint --template ./${OUTDIR}/buf.gen.yaml --config ./${OUTDIR}/buf.yaml

mv ./proto/tendermint/abci/types.pb.go ./abci/types

echo "proto files have been compiled"

echo "removing copied files"

find ${OUTDIR}/proto/tendermint/ -name *.proto \
	| sed "s/$OUTDIR\/\(.*\)/\1/g" \
	| xargs -I {} rm {}

rm -rf ${OUTDIR}
