#! /bin/bash
set -eu

N=$1

cd "$GOPATH/src/github.com/tendermint/tendermint"

manual_peers="$(test/p2p/ip.sh 1):46656"
for i in $(seq 2 $N); do
	manual_peers="$manual_peers,$(test/p2p/ip.sh $i):46656"
done
echo "$manual_peers"
