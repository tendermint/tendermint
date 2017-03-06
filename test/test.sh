#! /bin/bash
set -eu

# Get the directory of where this script is.
SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
DIR="$( cd -P "$( dirname "$SOURCE" )" && pwd )"

LOGS_DIR="$DIR/logs"
echo
echo "* [$(date +"%T")] cleaning up $LOGS_DIR"
rm -rf "$LOGS_DIR"
mkdir -p "$LOGS_DIR"

set +e
echo
echo "* [$(date +"%T")] removing run_test container"
docker rm -vf run_test
set -e

set +u
if [[ "$CIRCLECI" == true ]]; then
	echo
	echo "* [$(date +"%T")] starting rsyslog container"
	docker rm -f rsyslog || true
	docker run -d -v "$LOGS_DIR:/var/log/" -p 127.0.0.1:5514:514/udp --name rsyslog voxxit/rsyslog
fi

if [[ "$SKIP_BUILD" == "" ]]; then
	echo
	echo "* [$(date +"%T")] building docker image"
	bash "$DIR/docker/build.sh"
fi

echo
echo "* [$(date +"%T")] running go tests and app tests in docker container"
# sometimes its helpful to mount the local test folder
# -v $DIR:/go/src/github.com/tendermint/tendermint/test
if [[ "$CIRCLECI" == true ]]; then
	docker run --name run_test -e CIRCLECI=true -t tester bash test/run_test.sh
else
	docker run --name run_test -t tester bash test/run_test.sh
fi

# copy the coverage results out of docker container
docker cp run_test:/go/src/github.com/tendermint/tendermint/coverage.txt .

# test basic network connectivity
# by starting a local testnet and checking peers connect and make blocks
echo
echo "* [$(date +"%T")] running p2p tests on a local docker network"
bash "$DIR/p2p/test.sh" tester

# only run the cloud benchmark for releases
BRANCH=$(git rev-parse --abbrev-ref HEAD)
if [[ $(echo "$BRANCH" | grep "release-") != "" ]]; then
	echo
	echo "TODO: run network tests"
	#echo "* branch $BRANCH; running mintnet/netmon throughput benchmark"
	# TODO: replace mintnet
	#bash "$DIR/net/test.sh"
fi
