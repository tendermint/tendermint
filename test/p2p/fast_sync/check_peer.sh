#! /bin/bash
set -eu
set -o pipefail

IPV=$1
ID=$2

###########################################
#
# Wait for peer to catchup to other peers
#
###########################################

addr=$(test/p2p/address.sh $IPV $ID 26657)
peerID=$(( $(($ID % 4)) + 1  )) # 1->2 ... 3->4 ... 4->1
peer_addr=$(test/p2p/address.sh $IPV $peerID 26657)

# get another peer's height
h1=`curl -s $peer_addr/status | jq .result.sync_info.latest_block_height | jq fromjson`

# get another peer's state
root1=`curl -s $peer_addr/status | jq .result.sync_info.latest_app_hash`

echo "Other peer is on height $h1 with state $root1"
echo "Waiting for peer $ID to catch up"

# wait for it to sync to past its previous height
set +e
set +o pipefail
h2="0"
while [[ "$h2" -lt "$(($h1+3))" ]]; do
	sleep 1
	h2=`curl -s $addr/status | jq .result.sync_info.latest_block_height | jq fromjson`
	echo "... $h2"
done

# check the app hash
root2=`curl -s $addr/status | jq .result.sync_info.latest_app_hash`

if [[ "$root1" != "$root2" ]]; then
	echo "App hash after fast sync does not match. Got $root2; expected $root1"
	exit 1
fi
echo "... fast sync successful"
