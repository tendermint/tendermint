#!/usr/bin/env bash
#
# This is a convenience script that takes a list of testnet manifests
# as arguments and runs each one of them sequentially. If a testnet
# fails, the container logs are dumped to stdout along with the testnet
# manifest.
#
# This is mostly used to run generated networks in nightly CI jobs.
#

# Don't set -e, since we explicitly check status codes ourselves.
set -u

if [[ $# == 0 ]]; then
	echo "Usage: $0 [MANIFEST...]" >&2
	exit 1
fi

for MANIFEST in "$@"; do
	START=$SECONDS
	echo "==> Running testnet $MANIFEST..."
	./build/runner -f "$MANIFEST"

	if [[ $? -ne 0 ]]; then
		echo "==> Testnet $MANIFEST failed, dumping manifest..."
		cat "$MANIFEST"

		echo "==> Dumping container logs for $MANIFEST..."
		./build/runner -f "$MANIFEST" logs
		exit 1
	fi

	echo "==> Completed testnet $MANIFEST in $(( SECONDS - START ))s"
	echo ""
done
