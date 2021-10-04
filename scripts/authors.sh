#! /bin/bash

tag=$1


if [[ ! -z "$tag" ]]; then
	git log master..$tag | grep Author | sort | uniq
else
cat << EOF
Usage:
	./authors.sh <tag>
		Print a list of all authors who have committed to develop since the supplied tagged version.
EOF
fi

