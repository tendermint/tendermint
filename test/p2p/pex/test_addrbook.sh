#! /bin/bash
set -eu

DOCKER_IMAGE=$1
NETWORK_NAME=$2
IPV=$3
N=$4
PROXY_APP=$5

ID=1

echo "----------------------------------------------------------------------"
echo "Testing pex creates the addrbook and uses it if persistent_peers are not provided"
echo "(assuming peers are started with pex enabled)"

CLIENT_NAME="pex_addrbook_$ID"

echo "1. restart peer $ID"
docker stop "local_testnet_$ID"
echo "stopped local_testnet_$ID"
# preserve addrbook.json
docker cp "local_testnet_$ID:/go/src/github.com/tendermint/tendermint/test/p2p/data/mach0/config/addrbook.json" "/tmp/addrbook.json"
set +e #CIRCLE
docker rm -vf "local_testnet_$ID"
set -e

# NOTE that we do not provide persistent_peers
bash test/p2p/peer.sh "$DOCKER_IMAGE" "$NETWORK_NAME" $IPV "$ID" "$PROXY_APP" "--p2p.pex --rpc.unsafe --mode validator"
echo "started local_testnet_$ID"

# if the client runs forever, it means addrbook wasn't saved or was empty
bash test/p2p/client.sh "$DOCKER_IMAGE" "$NETWORK_NAME" "$IPV" "$CLIENT_NAME" "test/p2p/pex/check_peer.sh $IPV $ID $N"

# Now we know that the node is up.

docker cp "/tmp/addrbook.json" "local_testnet_$ID:/go/src/github.com/tendermint/tendermint/test/p2p/data/mach0/config/addrbook.json"
echo "with the following addrbook:"
cat /tmp/addrbook.json
# exec doesn't work on circle
# docker exec "local_testnet_$ID" cat "/go/src/github.com/tendermint/tendermint/test/p2p/data/mach0/config/addrbook.json"
echo ""

echo "----------------------------------------------------------------------"
echo "Testing other peers connect to us if we have neither persistent_peers nor the addrbook"
echo "(assuming peers are started with pex enabled)"

CLIENT_NAME="pex_no_addrbook_$ID"

echo "1. restart peer $ID"
docker stop "local_testnet_$ID"
echo "stopped local_testnet_$ID"
set +e #CIRCLE
docker rm -vf "local_testnet_$ID"
set -e

# NOTE that we do not provide persistent_peers
bash test/p2p/peer.sh "$DOCKER_IMAGE" "$NETWORK_NAME" $IPV "$ID" "$PROXY_APP" "--p2p.pex --rpc.unsafe --mode validator"
echo "started local_testnet_$ID"

# if the client runs forever, it means other peers have removed us from their books (which should not happen)
bash test/p2p/client.sh "$DOCKER_IMAGE" "$NETWORK_NAME" "$IPV" "$CLIENT_NAME" "test/p2p/pex/check_peer.sh $IPV $ID $N"

echo "----------------------------------------------------------------------"
echo "Testing other peers connect to us if we have seednode"
echo "(assuming peers are started with pex enabled)"

CLIENT_NAME="pex_no_addrbook_$ID"
SEED_ADDR=$(test/p2p/address.sh $IPV $ID 26657 $DOCKER_IMAGE)

for i in `seq 2 $N`; do
	docker exec "local_testnet_$i" rm "/go/src/github.com/tendermint/tendermint/test/p2p/data/mach$((i-1))/config/addrbook.json"
	echo "1. restart peer with seeds, no persi $i"
	docker stop "local_testnet_$i"
	echo "stopped local_testnet_$i"
	docker rm -f "local_testnet_$i"
	bash test/p2p/peer.sh "$DOCKER_IMAGE" "$NETWORK_NAME" $IPV "$i" "$PROXY_APP" "--p2p.pex --rpc.unsafe --mode validator --p2p.seeds $SEED_ADDR"
	echo "started local_testnet_$i"
done

echo "1. restart peer $ID"
docker stop "local_testnet_$ID"
echo "stopped local_testnet_$ID"
set +e #CIRCLE
docker rm -vf "local_testnet_$ID"
set -e

PERSISTENT_PEERS="$(test/p2p/address.sh $IPV 1 26656 $DOCKER_IMAGE)"
for j in `seq 2 $N`; do
	PERSISTENT_PEERS="$PERSISTENT_PEERS,$(test/p2p/address.sh $IPV $j 26656 $DOCKER_IMAGE)"
done

# NOTE that we do not provide persistent_peers
bash test/p2p/peer.sh "$DOCKER_IMAGE" "$NETWORK_NAME" $IPV "$ID" "$PROXY_APP" "--p2p.pex --rpc.unsafe --mode seednode --p2p.seed_mode --p2p.persistent_peers $PERSISTENT_PEERS"
echo "started local_testnet_$ID"

for i in `seq 2 $N`; do
	echo "1. restart peer with seeds, no persi $i"
	# if the client runs forever, it means other peers have removed us from their books (which should not happen)
	bash test/p2p/client.sh "$DOCKER_IMAGE" "$NETWORK_NAME" "$IPV" "$CLIENT_NAME" "test/p2p/pex/check_peer.sh $IPV $i $N"
done

# Now we know that the node is up.

echo ""
echo "PASS"
echo ""
