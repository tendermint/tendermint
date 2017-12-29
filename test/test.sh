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

echo
echo "* [$(date +"%T")] starting rsyslog container"
docker rm -f rsyslog || true
docker run -d -v "$LOGS_DIR:/var/log/" -p 127.0.0.1:5514:514/udp --name rsyslog voxxit/rsyslog

pwd

#TODO
## copy the coverage results out of docker container
#docker cp run_test:/go/src/github.com/tendermint/tendermint/coverage.txt .
