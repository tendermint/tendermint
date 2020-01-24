#! /bin/bash
set -eu

DOCKER_IMAGE=$1
NETWORK_NAME=$2
IPV=$3
NUM_OF_PEERS=$4
NUM_OF_CRASHES=$5

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
