#! /bin/bash
set -eu

DOCKER_IMAGE=$1
NETWORK_NAME=$2
NUM_OF_PEERS=$3
NUM_OF_CRASHES=$4

cd "$GOPATH/src/github.com/tendermint/tendermint"

###############################################################
# NUM_OF_CRASHES times:
# 	kill all peers
# 	bring them back online
# 	wait for them to sync and check that they are making progress
###############################################################

for i in $(seq 1 "$NUM_OF_CRASHES"); do
  # kill all peers
  for i in $(seq 1 "$NUM_OF_PEERS"); do
    docker rm -vf "local_testnet_$i"
  done

  # restart all peers
  SEEDS="$(test/p2p/ip.sh 1):46656"
  for j in $(seq 2 "$NUM_OF_PEERS"); do
    SEEDS="$SEEDS,$(test/p2p/ip.sh "$j"):46656"
  done
  for i in $(seq 1 "$NUM_OF_PEERS"); do
    bash test/p2p/peer.sh "$DOCKER_IMAGE" "$NETWORK_NAME" "$i" "$SEEDS"
  done

  bash test/p2p/client.sh "$DOCKER_IMAGE" "$NETWORK_NAME" kill_all "test/p2p/kill_all/check_peers.sh $NUM_OF_PEERS"
done

echo ""
echo "PASS"
echo ""
