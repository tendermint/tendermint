#! /bin/bash
set -eu

DOCKER_IMAGE=$1
NETWORK_NAME=$2
ID=$3
N=$4
PROXY_APP=$5

###############################################################
# this runs on each peer:
# 	kill peer
# 	bring it back online via fast sync
# 	wait for it to sync and check the app hash
###############################################################


echo "Testing fastsync on node $ID"

# kill peer
set +e # circle sigh :(
docker rm -vf local_testnet_$ID
set -e

# restart peer - should have an empty blockchain
SEEDS="$(test/p2p/ip.sh 1):46656"
for j in `seq 2 $N`; do
	SEEDS="$SEEDS,$(test/p2p/ip.sh $j):46656"
done
bash test/p2p/peer.sh $DOCKER_IMAGE $NETWORK_NAME $ID $PROXY_APP $SEEDS

# wait for peer to sync and check the app hash
bash test/p2p/client.sh $DOCKER_IMAGE $NETWORK_NAME fs_$ID "test/p2p/fast_sync/check_peer.sh $ID"

echo ""
echo "PASS"
echo ""

