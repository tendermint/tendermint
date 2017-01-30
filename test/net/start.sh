#! /bin/bash
set -eu

# start a testnet and benchmark throughput using mintnet+netmon via the network_testing repo

DATACENTER=single
VALSETSIZE=4
BLOCKSIZE=8092
TX_SIZE=200
NTXS=$((BLOCKSIZE*4))
RESULTSDIR=results
CLOUD_PROVIDER=digitalocean

set +u
if [[ "$MACH_PREFIX" == "" ]]; then
	MACH_PREFIX=mach
fi
set -u

cd "$GOPATH/src/github.com/tendermint/network_testing"
echo "... running network test $(pwd)"
TMHEAD=$(git rev-parse --abbrev-ref HEAD) TM_IMAGE="tendermint/tendermint" bash experiments/exp_throughput.sh $DATACENTER $VALSETSIZE $BLOCKSIZE $TX_SIZE $NTXS $MACH_PREFIX $RESULTSDIR $CLOUD_PROVIDER

# TODO: publish result!

# cleanup

echo "... destroying machines"
mintnet destroy --machines $MACH_PREFIX[1-$VALSETSIZE]
