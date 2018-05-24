#!/bin/bash

DOCKER_NIX_IMAGE=$1
CONTAINER=$(docker create $DOCKER_NIX_IMAGE)

echo $CONTAINER
rm -rf build
mkdir build
docker cp "$CONTAINER:/tendermint/build/tendermint" build
docker rm $CONTAINER
docker rmi $DOCKER_NIX_IMAGE
