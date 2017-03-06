#! /bin/bash
set -eu

DOCKER_IMAGE=$1
NETWORK_NAME=$2
N=$3
PROXY_APP=$4

ID=1

cd $GOPATH/src/github.com/tendermint/tendermint

echo "----------------------------------------------------------------------"
echo "Testing full network connection using one /dial_seeds call"
echo "(assuming peers are started with pex enabled)"

# stop the existing testnet and remove local network
set +e
bash test/p2p/local_testnet_stop.sh $NETWORK_NAME $N
set -e

# start the testnet on a local network
# NOTE we re-use the same network for all tests
SEEDS=""
bash test/p2p/local_testnet_start.sh $DOCKER_IMAGE $NETWORK_NAME $N $PROXY_APP $SEEDS



# dial seeds from one node
CLIENT_NAME="dial_seeds"
bash test/p2p/client.sh $DOCKER_IMAGE $NETWORK_NAME $CLIENT_NAME "test/p2p/pex/dial_seeds.sh $N"

# test basic connectivity and consensus
# start client container and check the num peers and height for all nodes
CLIENT_NAME="dial_seeds_basic"
bash test/p2p/client.sh $DOCKER_IMAGE $NETWORK_NAME $CLIENT_NAME "test/p2p/basic/test.sh $N"
