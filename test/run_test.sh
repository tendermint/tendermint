#! /bin/bash
set -e

echo `pwd`

BRANCH=`git rev-parse --abbrev-ref HEAD`
echo "Current branch: $BRANCH"

# go test --race github.com/tendermint/tendermint/...
make test_race

# run the broadcast_tx tests
bash test/broadcast_tx/test.sh

if [[ "$BRANCH" == "master" || "$BRANCH" == "staging" ]]; then
	echo ""
	echo "* branch $BRANCH; testing libs"
	# checkout every github.com/tendermint dir and run its tests
	bash test/test_libs.sh

	# TODO: mintnet/netmon
fi
