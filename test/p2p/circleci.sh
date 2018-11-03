#! /bin/bash
set -eux

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

echo
echo "* [$(date +"%T")] starting rsyslog container"
docker rm -f rsyslog || true
docker run -d -v "$LOGS_DIR:/var/log/" -p 127.0.0.1:5514:514/udp --name rsyslog voxxit/rsyslog

set +u
if [[ "$SKIP_BUILD" == "" ]]; then
	echo
	echo "* [$(date +"%T")] building docker image"
	bash "$DIR/../docker/build.sh"
fi

echo
echo "* [$(date +"%T")] running p2p tests on a local docker network"
bash "$DIR/../p2p/test.sh" tester

echo
echo "* [$(date +"%T")] copying log files out of docker container into $LOGS_DIR"
docker cp rsyslog:/var/log $LOGS_DIR
