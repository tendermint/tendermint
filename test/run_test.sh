#! /bin/bash
set -e

pwd

BRANCH=$(git rev-parse --abbrev-ref HEAD)
echo "Current branch: $BRANCH"

# run the go unit tests with coverage
bash test/test_cover.sh

# run the app tests using bash
bash test/app/test.sh

# run the persistence tests using bash
bash test/persist/test.sh

if [[ "$BRANCH" == "master" || $(echo "$BRANCH" | grep "release-") != "" ]]; then
	echo ""
	echo "* branch $BRANCH; testing libs"
	# checkout every github.com/tendermint dir and run its tests
	bash test/test_libs.sh
fi
