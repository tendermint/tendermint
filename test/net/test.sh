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

export TMHEAD=`git rev-parse --abbrev-ref HEAD`
export TM_IMAGE="tendermint/tmbase"

# grab glide for dependency mgmt
go get github.com/Masterminds/glide

# grab network monitor, install mintnet, netmon
# these might err 
echo "... fetching repos. ignore go get errors"
set +e
go get github.com/tendermint/network_testing
go get github.com/tendermint/mintnet
go get github.com/tendermint/netmon
set -e

# install vendored deps
echo "GOPATH $GOPATH"

cd $GOPATH/src/github.com/tendermint/mintnet
echo "... install mintnet dir $(pwd)"
glide install
go install
cd $GOPATH/src/github.com/tendermint/netmon
echo "... install netmon dir $(pwd)"
glide install
go install

cd $GOPATH/src/github.com/tendermint/network_testing
echo "... running network test $(pwd)"
bash experiments/exp_throughput.sh $DATACENTER $VALSETSIZE $BLOCKSIZE $TX_SIZE $NTXS $MACH_PREFIX $RESULTSDIR $CLOUD_PROVIDER

# TODO: publish result!

# cleanup

echo "... destroying machines"
mintnet destroy --machines $MACH_PREFIX[1-$VALSETSIZE] 


