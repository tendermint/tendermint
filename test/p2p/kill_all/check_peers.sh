#! /bin/bash
set -eu

NUM_OF_PEERS=$1

# how many attempts for each peer to catch up by height
MAX_ATTEMPTS_TO_CATCH_UP=20

echo "Waiting for nodes to come online"
set +e
for i in $(seq 1 "$NUM_OF_PEERS"); do
  addr=$(test/p2p/ip.sh "$i"):46657
  curl -s "$addr/status" > /dev/null
  ERR=$?
  while [ "$ERR" != 0 ]; do
    sleep 1
    curl -s "$addr/status" > /dev/null
    ERR=$?
  done
  echo "... node $i is up"
done
set -e

# get the first peer's height
addr=$(test/p2p/ip.sh 1):46657
h1=$(curl -s "$addr/status" | jq .result.latest_block_height)
echo "1st peer is on height $h1"

echo "Waiting until other peers reporting a height higher than the 1st one"
for i in $(seq 2 "$NUM_OF_PEERS"); do
  attempt=1
  hi=0

  while [[ $hi -le $h1 ]] ; do
    addr=$(test/p2p/ip.sh "$i"):46657
    hi=$(curl -s "$addr/status" | jq .result.latest_block_height)

    echo "... peer $i is on height $hi"

    ((attempt++))
    if [ "$attempt" -ge $MAX_ATTEMPTS_TO_CATCH_UP ] ; then
      echo "$attempt unsuccessful attempts were made to catch up"
      curl -s "$addr/dump_consensus_state" | jq .result
      exit 1
    fi

    sleep 1
  done
done
