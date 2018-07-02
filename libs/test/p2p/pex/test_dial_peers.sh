#! /bin/bash
set -eu

DOCKER_IMAGE=$1
NETWORK_NAME=$2
N=$3
PROXY_APP=$4

ID=1

cd $GOPATH/src/github.com/tendermint/tendermint

echo "----------------------------------------------------------------------"
echo "Testing full network connection using one /dial_peers call"
echo "(assuming peers are started with pex enabled)"

# stop the existing testnet and remove local network
set +e
bash test/p2p/local_testnet_stop.sh $NETWORK_NAME $N
set -e

# start the testnet on a local network
# NOTE we re-use the same network for all tests
bash test/p2p/local_testnet_start.sh $DOCKER_IMAGE $NETWORK_NAME $N $PROXY_APP ""

PERSISTENT_PEERS="\"$(test/p2p/ip_plus_id.sh 1 $DOCKER_IMAGE):26656\""
for i in $(seq 2 $N); do
	PERSISTENT_PEERS="$PERSISTENT_PEERS,\"$(test/p2p/ip_plus_id.sh $i $DOCKER_IMAGE):26656\""
done
echo "$PERSISTENT_PEERS"

# dial peers from one node
CLIENT_NAME="dial_peers"
bash test/p2p/client.sh $DOCKER_IMAGE $NETWORK_NAME $CLIENT_NAME "test/p2p/pex/dial_peers.sh $N $PERSISTENT_PEERS"

# test basic connectivity and consensus
# start client container and check the num peers and height for all nodes
CLIENT_NAME="dial_peers_basic"
bash test/p2p/client.sh $DOCKER_IMAGE $NETWORK_NAME $CLIENT_NAME "test/p2p/basic/test.sh $N"
