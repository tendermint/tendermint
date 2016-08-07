#! /bin/bash
set -eu

DOCKER_IMAGE=$1
NETWORK_NAME=$2

cd $GOPATH/src/github.com/tendermint/tendermint

# create docker network
docker network create --driver bridge --subnet 172.57.0.0/16 $NETWORK_NAME

N=4
seeds="172.57.0.101:46656"
for i in `seq 2 $N`; do
	seeds="$seeds,172.57.0.$((100+$i)):46656"
done
echo "Seeds: $seeds"

for i in `seq 1 $N`; do
	# start tendermint container
	docker run -d \
		--net=$NETWORK_NAME \
		--ip=172.57.0.$((100+$i)) \
		--name local_testnet_$i \
		--entrypoint tendermint \
		-e TMROOT=/go/src/github.com/tendermint/tendermint/test/p2p/data/mach$i/core \
		$DOCKER_IMAGE node --seeds $seeds --proxy_app=dummy
done
