#! /bin/bash
set -eu

# Top Level Testing Script
# See the github.com/tendermint/tendermint/test/README.md

echo ""
echo "* building docker file"
docker build -t tester -f ./test/Dockerfile .

echo ""
echo "* running go tests and app tests"
docker run -t tester bash test/run_test.sh

# test basic network connectivity
# by starting a local testnet and checking peers connect and make blocks
echo ""
echo "* running basic peer tests"
bash test/p2p/test.sh tester

# only run the cloud benchmark for releases
BRANCH=`git rev-parse --abbrev-ref HEAD`
if [[ $(echo "$BRANCH" | grep "release-") != "" ]]; then
	echo ""
	echo "* branch $BRANCH; running mintnet/netmon throughput benchmark"
	bash test/net/test.sh
fi
