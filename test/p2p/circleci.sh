#! /bin/bash
set -eux

# Take IP version as parameter
IPV="${1:-4}"

# Get the directory of where this script is.
SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
DIR="$( cd -P "$( dirname "$SOURCE" )" && pwd )"

# Enable IPv6 support in Docker daemon
if [[ "$IPV" == "6" ]]; then
	echo
	echo "* [$(date +"%T")] enabling IPv6 stack in Docker daemon"
	cat <<'EOF' | sudo tee /etc/docker/daemon.json
{
	"ipv6": true,
	"fixed-cidr-v6": "2001:db8:1::/64"
}
EOF
	sudo service docker restart
fi

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
echo "* [$(date +"%T")] running IPv$IPV p2p tests on a local docker network"
bash "$DIR/../p2p/test.sh" tester $IPV

echo
echo "* [$(date +"%T")] copying log files out of docker container into $LOGS_DIR"
docker cp rsyslog:/var/log $LOGS_DIR
