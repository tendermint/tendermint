#!/bin/env bash
set -euo pipefail

find ${OUTDIR}/proto/tendermint/ -name *.proto \
	| sed "s/$OUTDIR\/\(.*\)/\1/g" \
	| xargs -I {} rm {}

rm -rf ${OUTDIR}
