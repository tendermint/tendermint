#! /bin/bash

# integrations test
# this is the script run by eg CircleCI.
# It creates a docker container,
# installs the dependencies,
# and runs the tests.
# If we pushed to STAGING or MASTER,
# it will also run the tests for all dependencies

echo ""
echo "* building docker file"
docker build -t tester -f ./test/Dockerfile .

echo ""
echo "* running go tests and broadcast tests"
docker run -t tester bash test/run_test.sh

# test basic network connectivity
# by starting a local testnet and checking peers connect and make blocks
echo ""
echo "* running basic peer tests"
bash test/p2p/test.sh tester

BRANCH=`git rev-parse --abbrev-ref HEAD`
if [[ "$BRANCH" == "master" || $(echo "$BRANCH" | grep "release-") != "" ]]; then
	echo ""
	echo "* branch $BRANCH; running mintnet/netmon throughput benchmark"
	bash test/net/test.sh
fi
