#! /bin/bash
set -eu

DOCKER_IMAGE=$1
NETWORK_NAME=$2

cd $GOPATH/src/github.com/tendermint/tendermint

# create docker network
docker network create --driver bridge --subnet 172.57.0.0/16 $NETWORK_NAME

N=4
seeds="$(test/p2p/ip.sh 1):46656"
for i in `seq 2 $N`; do
	seeds="$seeds,$(test/p2p/ip.sh $i):46656"
done
echo "Seeds: $seeds"

for i in `seq 1 $N`; do
	bash test/p2p/peer.sh $DOCKER_IMAGE $NETWORK_NAME $i $seeds
done
