#!/bin/env bash
set -euo pipefail
: ${VERS:=master}

# Get shortened ref of commit
REF=$(curl -H "Accept: application/vnd.github.v3.sha" -qL \
  "https://api.github.com/repos/tendermint/spec/commits/${VERS}" \
  | cut -c -7)

readonly OUTDIR="tendermint-spec-${REF}"
curl -qL "https://api.github.com/repos/tendermint/spec/tarball/${REF}" | tar -xzf - ${OUTDIR}/

cp -r ${OUTDIR}/proto/tendermint/* ./proto/tendermint
cp -r ${OUTDIR}/third_party/** ./third_party

echo ${OUTDIR}
