#!/usr/bin/env bash
#
# This script is used to check which commit a testnet failed
#
# For example, to examine which commit from the last 10 failed 
# with "networks/gen/gen-0000.toml" you would run the following
# from the root directory:
#  
#    git bisect start HEAD HEAD~10
#    git bisect run test/e2e/scripts/bisect.sh test/e2e/networks/gen/gen-0000.toml 4
#    git bisect reset 

set -euo pipefail

if [[ $# == 0 ]]; then
	echo "Usage: $0 [MANIFEST] [COUNT]" >&2
	exit 125
fi

MANIFEST=$1
COUNT=$2

cd ..

if make; then
    for i in {0..$COUNT}; do
		if ! ./build/runner -f "$MANIFEST"; then
            ./build/runner -f "$MANIFEST" cleanup
			exit 1
		fi
	done
    exit 1
else
    exit 125