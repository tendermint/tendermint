#!/usr/bin/env bash
#
# This script is used to check failing testnets, checking for deterministic failures
# and saving each failed testnet log to file

set -euo pipefail

if [[ $# == 0 ]]; then 
	echo "Usage: $0 [MANIFEST...]" >&2
	exit 1
fi

FAILED=()

for MANIFEST in "$@"; do
	echo "==> Running testnet $MANIFEST..."

	FAILED_COUNT=0

	for i in {1..3}; do
		if ! ./build/runner -f "$MANIFEST"; then
			echo "==> Testnet $MANIFEST failed"
			((FAILED_COUNT=FAILED_COUNT+1))
            ./build/runner -f "$MANIFEST" logs >> ${MANIFEST%.toml}-$i.txt
		fi
		echo ""
	done

	if [[ $FAILED_COUNT > 0 ]]; then
		for i in {3..5}; do
			if ! ./build/runner -f "$MANIFEST"; then
				echo "==> Testnet $MANIFEST failed"
				((FAILED_COUNT=FAILED_COUNT+1))
                ./build/runner -f "$MANIFEST" logs >> ${MANIFEST%.toml}-$i.txt
			fi
			echo ""
		done
	fi

	if [[ $FAILED_COUNT > 0 ]]; then 
		echo "==> Testnet $MANIFEST failed $FAILED_COUNT out of 5 times"
		./build/runner -f "$MANIFEST" cleanup
        FAILED+=("$MANIFEST ($FAILED_COUNT times)")
	else
		echo "==> Testnet $MANIFEST completed without failing"
	fi
	echo ""
done

if [[ ${#FAILED[@]} -ne 0 ]]; then
	echo "${#FAILED[@]} testnets failed:"
	for MANIFEST in "${FAILED[@]}"; do
		echo "- $MANIFEST"
	done
	exit 1
else
	echo "All testnets successful"
fi
