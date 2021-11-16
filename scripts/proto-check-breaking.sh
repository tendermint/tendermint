#!/usr/bin/env bash
set -euo pipefail

: ${VERS:=master}
: ${AGAINST:='.git#branch=master'}

echo "fetching proto files"
OUTDIR=$(VERS=$VERS sh ./scripts/fetch-spec-protos.sh)

buf breaking --against $AGAINST

OUTDIR=$OUTDIR sh ./scripts/clear-spec-protos.sh
