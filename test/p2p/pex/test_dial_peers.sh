#! /bin/bash
set -eu

DOCKER_IMAGE=$1
NETWORK_NAME=$2
IPV=$3
N=$4
PROXY_APP=$5

ID=1

echo "----------------------------------------------------------------------"
echo "Testing full network connection using one /dial_peers call"
echo "(assuming peers are started with pex enabled)"

# stop the existing testnet and remove local network
set +e
bash test/p2p/local_testnet_stop.sh $NETWORK_NAME $N
set -e

# start the testnet on a local network
# NOTE we re-use the same network for all tests
bash test/p2p/local_testnet_start.sh $DOCKER_IMAGE $NETWORK_NAME $IPV $N $PROXY_APP ""

PERSISTENT_PEERS="\"$(test/p2p/address.sh $IPV 1 26656 $DOCKER_IMAGE)\""
for i in $(seq 2 $N); do
	PERSISTENT_PEERS="$PERSISTENT_PEERS,\"$(test/p2p/address.sh $IPV $i 26656 $DOCKER_IMAGE)\""
done
echo "$PERSISTENT_PEERS"

# dial peers from one node
CLIENT_NAME="dial_peers"
bash test/p2p/client.sh $DOCKER_IMAGE $NETWORK_NAME $IPV $CLIENT_NAME "test/p2p/pex/dial_peers.sh $IPV $N $PERSISTENT_PEERS"

# test basic connectivity and consensus
# start client container and check the num peers and height for all nodes
CLIENT_NAME="dial_peers_basic"
bash test/p2p/client.sh $DOCKER_IMAGE $NETWORK_NAME $IPV $CLIENT_NAME "test/p2p/basic/test.sh $IPV $N"
