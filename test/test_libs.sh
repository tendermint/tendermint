#! /bin/bash
set -e

# set glide.lock path
if [[ "$GLIDE" == "" ]]; then
	GLIDE=$GOPATH/src/github.com/tendermint/tendermint/glide.lock
fi

# get vendored commit for given lib

####################
# libs we depend on
####################

# All libs should define `make test` and `make get_vendor_deps`
LIBS_TEST=(tmlibs go-wire go-crypto abci)

DIR=$(pwd)
for lib in "${LIBS_MAKE_TEST[@]}"; do

	# checkout vendored version of lib
	bash scripts/glide/checkout.sh "$GLIDE" "$lib"

	echo "Testing $lib ..."
	cd "$GOPATH/src/github.com/tendermint/$lib"
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
