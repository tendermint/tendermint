#! /bin/bash
set -u

ID=$1
N=$2

addr=$(test/p2p/ip.sh "$ID"):26657

echo "2. wait until peer $ID connects to other nodes using pex reactor"
peers_count="0"
while [[ "$peers_count" -lt "$((N-1))" ]]; do
  sleep 1
  peers_count=$(curl -s "$addr/net_info" | jq ".result.peers | length")
  echo "... peers count = $peers_count, expected = $((N-1))"
done

echo "... successful"
