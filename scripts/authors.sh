#! /bin/bash

ref=$1

if [[ ! -z "$ref" ]]; then
	git log master..$ref | grep Author | sort | uniq
else
cat << EOF
Usage:
	./authors.sh <ref>
		Print a list of all authors who have committed to develop since the supplied commit ref. 
EOF
fi

