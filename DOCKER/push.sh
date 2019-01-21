#!/usr/bin/env bash
set -e

# By default, we build Tendermint's Docker image
IMAGE=${IMAGE:-"tendermint"}

# Get the tag from the version, or try to figure it out.
if [ -z "$TAG" ]; then
	TAG=$(awk -F\" '/TMCoreSemVer =/ { print $2; exit }' < ../version/version.go)
fi
if [ -z "$TAG" ]; then
		echo "Please specify a tag."
		exit 1
fi

TAG_NO_PATCH=${TAG%.*}

read -p "==> Push 3 docker images with the following tags (latest, $TAG, $TAG_NO_PATCH)? y/n" -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]
then
	docker push "tendermint/${IMAGE}:latest"
	docker push "tendermint/${IMAGE}:$TAG"
	docker push "tendermint/${IMAGE}:$TAG_NO_PATCH"
fi
