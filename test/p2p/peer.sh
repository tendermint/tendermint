#! /bin/bash
set -eu

DOCKER_IMAGE=$1
NETWORK_NAME=$2
ID=$3
APP_PROXY=$4

set +u
SEEDS=$5
set -u
if [[ "$SEEDS" != "" ]]; then
	SEEDS=" --seeds $SEEDS "
fi

echo "starting tendermint peer ID=$ID"
# start tendermint container on the network
docker run -d \
  --net=$NETWORK_NAME \
  --ip=$(test/p2p/ip.sh $ID) \
  --name local_testnet_$ID \
  --entrypoint tendermint \
  -e TMROOT=/go/src/github.com/tendermint/tendermint/test/p2p/data/mach$ID/core \
  $DOCKER_IMAGE node $SEEDS --proxy_app=$APP_PROXY
