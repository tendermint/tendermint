#! /bin/bash
set -eu
set -o pipefail

DOCKER_IMAGE=$1
NETWORK_NAME=$2
IPV=$3
ID=$4
N=$5
PROXY_APP=$6
ASSERT_NODE_UP=1
ASSERT_NODE_DOWN=0

###########################s####################################
# this runs on each peer:
# 	kill peer
# 	bring it back online with double_sign_check_height 10
# 	wait node is not run by double sign risk reduction
#
# 	kill peer
# 	bring it back online with double_sign_check_height 1
# 	pass double sign risk reduction, wait for it to sync and check the app hash
#
# 	kill peer
# 	bring it back online with double_sign_check_height 0
# 	wait for it to sync and check the app hash
###############################################################

echo "Testing double sign risk reduction on node $ID"

# kill peer
set +e
	docker rm -vf local_testnet_$ID
	set -e
	PERSISTENT_PEERS="$(test/p2p/address.sh $IPV 1 26656 $DOCKER_IMAGE)"
	for j in `seq 2 $N`; do
		PERSISTENT_PEERS="$PERSISTENT_PEERS,$(test/p2p/address.sh $IPV $j 26656 $DOCKER_IMAGE)"
	done

	# bring it back online with double_sign_check_height 10
	# wait node is not run by double sign risk reduction
	DSCH=10
	bash test/p2p/peer.sh $DOCKER_IMAGE $NETWORK_NAME $IPV $ID $PROXY_APP "--p2p.persistent_peers $PERSISTENT_PEERS --p2p.pex --rpc.unsafe --consensus.double_sign_check_height $DSCH"
	bash test/p2p/client.sh $DOCKER_IMAGE $NETWORK_NAME $IPV fs_$ID "test/p2p/dsrr/check_peer.sh $IPV $ID $ASSERT_NODE_DOWN"


	docker stop local_testnet_$ID
	docker rm local_testnet_$ID
	# bring it back online with double_sign_check_height 1
	# pass double sign risk reduction, wait for it to sync and check the app hash
	DSCH=1
	bash test/p2p/peer.sh $DOCKER_IMAGE $NETWORK_NAME $IPV $ID $PROXY_APP "--p2p.persistent_peers $PERSISTENT_PEERS --p2p.pex --rpc.unsafe --consensus.double_sign_check_height $DSCH"
	bash test/p2p/client.sh $DOCKER_IMAGE $NETWORK_NAME $IPV fs_$ID "test/p2p/dsrr/check_peer.sh $IPV $ID $ASSERT_NODE_UP"

	docker stop local_testnet_$ID
	docker rm local_testnet_$ID
	DSCH=0
	# bring it back online with double_sign_check_height 0
	# double sign risk reduction is not activated, wait for it to sync and check the app hash
	bash test/p2p/peer.sh $DOCKER_IMAGE $NETWORK_NAME $IPV $ID $PROXY_APP "--p2p.persistent_peers $PERSISTENT_PEERS --p2p.pex --rpc.unsafe --consensus.double_sign_check_height $DSCH"
	bash test/p2p/client.sh $DOCKER_IMAGE $NETWORK_NAME $IPV fs_$ID "test/p2p/dsrr/check_peer.sh $IPV $ID $ASSERT_NODE_UP"

	echo ""
	echo "PASS"
	echo ""
