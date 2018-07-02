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
	PERSISTENT_PEERS="$(test/p2p/ip_plus_id.sh 1 $DOCKER_IMAGE):26656"
	for j in `seq 2 $N`; do
		PERSISTENT_PEERS="$PERSISTENT_PEERS,$(test/p2p/ip_plus_id.sh $j $DOCKER_IMAGE):26656"
	done
	bash test/p2p/peer.sh $DOCKER_IMAGE $NETWORK_NAME $ID $PROXY_APP "--p2p.persistent_peers $PERSISTENT_PEERS --p2p.pex --rpc.unsafe"

	# wait for peer to sync and check the app hash
	bash test/p2p/client.sh $DOCKER_IMAGE $NETWORK_NAME fs_$ID "test/p2p/fast_sync/check_peer.sh $ID"

	echo ""
	echo "PASS"
	echo ""

