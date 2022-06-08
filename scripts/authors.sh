#! /bin/bash

set -euo pipefail

ref=$1

if [[ ! -z "$ref" ]]; then
	git log master..$ref | grep Author | sort | uniq
else
cat << EOF
Usage:
	./authors.sh <ref>
		Print a list of all authors who have committed to the codebase since the supplied commit ref. 
EOF
fi

