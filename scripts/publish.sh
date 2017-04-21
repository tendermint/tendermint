#!/usr/bin/env bash
set -eu

VERSION=$1 
DIST_DIR=$2 # ./build/dist

# Get the version from the environment, or try to figure it out.
if [ -z $VERSION ]; then
	VERSION=$(awk -F\" '/Version =/ { print $2; exit }' < version/version.go)
fi
if [ -z "$VERSION" ]; then
    echo "Please specify a version."
    exit 1
fi

# copy to s3
aws s3 cp --recursive ${DIST_DIR} s3://tendermint/${VERSION} --acl public-read --exclude "*" --include "*.zip" 
aws s3 cp ${DIST_DIR}/tendermint_${VERSION}_SHA256SUMS s3://tendermint/0.9.0 --acl public-read 

exit 0
