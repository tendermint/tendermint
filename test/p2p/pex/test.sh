#! /bin/bash
set -eu

DOCKER_IMAGE=$1
NETWORK_NAME=$2
N=$3
PROXY_APP=$4

cd $GOPATH/src/github.com/tendermint/tendermint

echo "Test reconnecting from the peer book"
bash test/p2p/pex/test_peerbook.sh $DOCKER_IMAGE $NETWORK_NAME $N $PROXY_APP

echo "Test connecting via /dial_seeds"
bash test/p2p/pex/test_dial_seeds.sh $DOCKER_IMAGE $NETWORK_NAME $N $PROXY_APP
