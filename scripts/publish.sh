#!/usr/bin/env bash
set -e

VERSION=$1 
DIST_DIR=./build/dist

# Get the version from the environment, or try to figure it out.
if [ -z $VERSION ]; then
	VERSION=$(awk -F\" 'TMCoreSemVer =/ { print $2; exit }' < version/version.go)
fi
if [ -z "$VERSION" ]; then
    echo "Please specify a version."
    exit 1
fi
echo "==> Copying ${DIST_DIR} to S3..."

# copy to s3
aws s3 cp --recursive ${DIST_DIR} s3://tendermint/binaries/tendermint/v${VERSION} --acl public-read 

exit 0
