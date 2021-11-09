#!/usr/bin/sh
set -eo pipefail

# This script appends the "option go_package" proto option to the file located at $FNAME.
# This option is not included in the proto files to allow the files to more easily
# be hosted in github.com/tendermint/spec and shared between other repos.
# This option specifies what the package will be named when imported by other packages.

FNAME=$1
MODNAME=$(echo $2| sed 's/\//\\\//g')
PACKAGE="$(dirname $FNAME | cut -c 2- | sed 's/\//\\\//g')"

sed -i "s/\(package tendermint.*\)/\1\n\noption go_package = \"$MODNAME$PACKAGE\";/g" $FNAME
