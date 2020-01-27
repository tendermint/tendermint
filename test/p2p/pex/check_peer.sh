#! /bin/bash
set -u

IPV=$1
ID=$2
N=$3

addr=$(test/p2p/address.sh $IPV "$ID" 26657)

echo "2. wait until peer $ID connects to other nodes using pex reactor"
peers_count="0"
while [[ "$peers_count" -lt "$((N-1))" ]]; do
  sleep 1
  peers_count=$(curl -s "$addr/net_info" | jq ".result.peers | length")
  echo "... peers count = $peers_count, expected = $((N-1))"
done

echo "... successful"
