#!/usr/bin/env bash
set -e

# Get the version from the environment, or try to figure it out.
if [ -z $VERSION ]; then
	VERSION=$(awk -F\" 'TMCoreSemVer =/ { print $2; exit }' < version/version.go)
fi
if [ -z "$VERSION" ]; then
    echo "Please specify a version."
    exit 1
fi
echo "==> Releasing version $VERSION..."

# Get the parent directory of where this script is.
SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
DIR="$( cd -P "$( dirname "$SOURCE" )/.." && pwd )"

# Change into that dir because we expect that.
cd "$DIR"

# Building binaries
sh -c "'$DIR/scripts/dist.sh'"

# Pushing binaries to S3
sh -c "'$DIR/scripts/publish.sh'"

# echo "==> Crafting a Github release"
# today=$(date +"%B-%d-%Y")
# ghr -b "https://github.com/tendermint/tendermint/blob/master/CHANGELOG.md#${VERSION//.}-${today,}" "v$VERSION" "$DIR/build/dist"

# Build and push Docker image

## Get SHA256SUM of the linux archive
SHA256SUM=$(shasum -a256 "${DIR}/build/dist/tendermint_${VERSION}_linux_amd64.zip" | awk '{print $1;}')

## Replace TM_VERSION and TM_SHA256SUM with the new values
sed -i -e "s/TM_VERSION .*/TM_VERSION $VERSION/g" "$DIR/DOCKER/Dockerfile"
sed -i -e "s/TM_SHA256SUM .*/TM_SHA256SUM $SHA256SUM/g" "$DIR/DOCKER/Dockerfile"
git commit -m "update Dockerfile" -a "$DIR/DOCKER/Dockerfile"
echo "==> TODO: update DOCKER/README.md (latest Dockerfile's hash is $(git rev-parse HEAD)) and copy it's content to https://store.docker.com/community/images/tendermint/tendermint"

pushd "$DIR/DOCKER"

## Build Docker image
TAG=$VERSION sh -c "'./build.sh'"

## Push Docker image
TAG=$VERSION sh -c "'./push.sh'"

popd

exit 0
