#!/bin/bash
set -eo pipefail

# This script appends the "option go_package" proto option to the file located at $FNAME.
# This option specifies what the package will be named when imported by other packages.
# This option is not directly included in the proto files to allow the files to more easily
# be hosted in github.com/tendermint/spec and shared between other repos.
# If the option is already specified in the file, it will be replaced using the
# arguments passed to this script.

FNAME="${1:?missing required .proto filename}"
MODNAME=$(echo $2| sed 's/\//\\\//g')
PACKAGE="$(dirname $FNAME | sed 's/^\.\/\(.*\)/\1/g' | sed 's/\//\\\//g')"
if [[ ! -z "$3" ]]; then
	PACKAGE="$(echo $3 | sed 's/\//\\\//g')"
fi


if ! grep -q 'option\s\+go_package\s\+=\s\+.*;' $FNAME; then
	sed -i "" "s/\(package tendermint.*\)/\1\n\noption go_package = \"$MODNAME\/$PACKAGE\";/g" $FNAME
else
	sed -i "" "s/option\s\+go_package\s\+=\s\+.*;/option go_package = \"$MODNAME\/$PACKAGE\";/g" $FNAME
fi
