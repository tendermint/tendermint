#! /bin/bash
set -eu

DOCKER_IMAGE=$1
NETWORK_NAME=$2
IPV=$3
NUM_OF_PEERS=$4
NUM_OF_CRASHES=$5
PROXY_APP=$6

###############################################################
# restart and reset half the peers as fullnode mode
# NUM_OF_CRASHES times:
#   restart all peers
# 	check chain is stuck, waiting until other peers reporting a height higher than the 1st one
# return half the peers to validator mode
###############################################################

HALF_SEQ=$((NUM_OF_PEERS/2+1))

for i in `seq $HALF_SEQ $NUM_OF_PEERS`; do
	echo "restart peer as fullnode $i"
	docker stop "local_testnet_$i"
	echo "stopped local_testnet_$i"
	docker rm -f "local_testnet_$i"
	bash test/p2p/peer.sh "$DOCKER_IMAGE" "$NETWORK_NAME" $IPV "$i" "$PROXY_APP" "--p2p.pex --rpc.unsafe --mode fullnode"
	echo "started local_testnet_$i"
done

set +e
for i in $(seq 1 "$NUM_OF_CRASHES"); do
  echo ""
  echo "Restarting fullnode peers! Take $i ..."

  # restart all peers
  for j in $(seq 1 "$NUM_OF_PEERS"); do
    docker stop "local_testnet_$j"
    docker start "local_testnet_$j"
  done

  bash test/p2p/client.sh "$DOCKER_IMAGE" "$NETWORK_NAME" "$IPV" kill_all_$i "test/p2p/kill_all/check_peers_fullnode.sh $IPV $NUM_OF_PEERS"
done
set -e

for i in `seq $HALF_SEQ $NUM_OF_PEERS`; do
	echo "restart peer as validator $i"
	docker stop "local_testnet_$i"
	echo "stopped local_testnet_$i"
	docker rm -f "local_testnet_$i"
	bash test/p2p/peer.sh "$DOCKER_IMAGE" "$NETWORK_NAME" $IPV "$i" "$PROXY_APP" "--p2p.pex --rpc.unsafe --mode validator"
	echo "started local_testnet_$i"
done

###############################################################
# NUM_OF_CRASHES times:
# 	restart all peers
# 	wait for them to sync and check that they are making progress
###############################################################

for i in $(seq 1 "$NUM_OF_CRASHES"); do
  echo ""
  echo "Restarting all peers! Take $i ..."

  # restart all peers
  for j in $(seq 1 "$NUM_OF_PEERS"); do
    docker stop "local_testnet_$j"
    docker start "local_testnet_$j"
  done

  bash test/p2p/client.sh "$DOCKER_IMAGE" "$NETWORK_NAME" "$IPV" kill_all_$i "test/p2p/kill_all/check_peers.sh $IPV $NUM_OF_PEERS"
done

echo ""
echo "PASS"
echo ""
