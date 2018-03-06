#! /bin/bash
set -ex

export PATH="$GOBIN:$PATH"

# Get the parent directory of where this script is.
SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
DIR="$( cd -P "$( dirname "$SOURCE" )/.." && pwd )"

####################
# libs we depend on
####################

# All libs should define `make test` and `make get_vendor_deps`
LIBS=(tmlibs go-wire go-crypto abci)
for lib in "${LIBS[@]}"; do
	# checkout vendored version of lib
	bash scripts/dep_utils/checkout.sh "$lib"

	echo "Testing $lib ..."
	cd "$GOPATH/src/github.com/tendermint/$lib"
	make get_tools
	make get_vendor_deps
	make test
	if [[ "$?" != 0 ]]; then
		echo "FAIL"
		exit 1
	fi
	cd "$DIR"
done

echo ""
echo "PASS"
