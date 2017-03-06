#! /bin/bash
set -u

NETWORK_NAME=$1
N=$2

for i in $(seq 1 "$N"); do
  docker stop "local_testnet_$i"
  docker rm -vf "local_testnet_$i"
done

docker network rm "$NETWORK_NAME"
