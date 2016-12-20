#! /bin/bash
set -eu

NUM_OF_PEERS=$1

ATTEMPTS_TO_CATCH_UP=5

heights_are_the_same=false
heights[1]=0
attempt=1

echo "Waiting for nodes to come online"
set +e
for i in $(seq 1 "$NUM_OF_PEERS"); do
  addr=$(test/p2p/ip.sh "$i"):46657
  curl -s "$addr/status" > /dev/null
  ERR=$?
  while [ "$ERR" != 0 ]; do
    sleep 1
    echo "$ERR"
    curl -s "$addr/status" > /dev/null
    ERR=$?
  done
  echo "... node $i is up"
done
set -e

while [[ $heights_are_the_same = false || ${heights[1]} -lt 1 ]] ; do
  echo "Waiting for peers to catch up by height"
  sleep 1

  for i in $(seq 1 "$NUM_OF_PEERS"); do
    addr=$(test/p2p/ip.sh "$i"):46657

    heights[$i]=$(curl -s "$addr/status" | jq .result[1].latest_block_height)
  done

  heights_are_the_same=true

  for i in $(seq 2 "$NUM_OF_PEERS"); do
    if [ "${heights[$((i - 1))]}" != "${heights[$i]}" ] ; then
      heights_are_the_same=false
      echo "Peers have different heights: ${heights[@]}"
      break
    fi
  done

  ((attempt++))
  if [ "$attempt" -ge $ATTEMPTS_TO_CATCH_UP ] ; then
    echo "$attempt unsuccessful attempts were made to catch up by height"
    exit 1
  fi
done

# check that peers have the same latest app hash
for i in $(seq 1 "$NUM_OF_PEERS"); do
  addr=$(test/p2p/ip.sh "$i"):46657

  roots[$i]=$(curl -s "$addr/status" | jq .result[1].latest_app_hash)
done

roots_are_the_same=true

for i in $(seq 2 "$NUM_OF_PEERS"); do
  if [ "${roots[$((i - 1))]}" != "${roots[$i]}" ] ; then
    roots_are_the_same=false
    break
  fi
done

if [ $roots_are_the_same = false ] ; then
  echo "Peers have different roots: ${roots[@]}"
  exit 1
fi
