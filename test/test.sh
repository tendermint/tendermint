#! /bin/bash
set -eu

# Top Level Testing Script
# See the github.com/tendermint/tendermint/test/README.md

echo ""
echo "* building docker image"
bash ./test/docker/build.sh

echo ""
echo "* running go tests and app tests in docker container"
# sometimes its helpful to mount the local test folder
# -v $GOPATH/src/github.com/tendermint/tendermint/test:/go/src/github.com/tendermint/tendermint/test
docker run --name run_test -t tester bash test/run_test.sh

# copy the coverage results out of docker container
docker cp run_test:/go/src/github.com/tendermint/tendermint/coverage.txt .

# test basic network connectivity
# by starting a local testnet and checking peers connect and make blocks
echo ""
echo "* running p2p tests on a local docker network"
bash test/p2p/test.sh tester

# only run the cloud benchmark for releases
BRANCH=`git rev-parse --abbrev-ref HEAD`
if [[ $(echo "$BRANCH" | grep "release-") != "" ]]; then
	echo ""
	echo "TODO: run network tests"
	#echo "* branch $BRANCH; running mintnet/netmon throughput benchmark"
	# TODO: replace mintnet 
	#bash test/net/test.sh
fi
