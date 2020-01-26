#! /bin/bash
set -eu

DOCKER_IMAGE=$1
NETWORK_NAME=$2
IPV=$3
ID=$4
N=$5
PROXY_APP=$6

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
	PERSISTENT_PEERS="$(test/p2p/address.sh $IPV 1 26656 $DOCKER_IMAGE)"
	for j in `seq 2 $N`; do
		PERSISTENT_PEERS="$PERSISTENT_PEERS,$(test/p2p/address.sh $IPV $j 26656 $DOCKER_IMAGE)"
	done
	bash test/p2p/peer.sh $DOCKER_IMAGE $NETWORK_NAME $IPV $ID $PROXY_APP "--p2p.persistent_peers $PERSISTENT_PEERS --p2p.pex --rpc.unsafe"

	# wait for peer to sync and check the app hash
	bash test/p2p/client.sh $DOCKER_IMAGE $NETWORK_NAME $IPV fs_$ID "test/p2p/fast_sync/check_peer.sh $IPV $ID"

	echo ""
	echo "PASS"
	echo ""

